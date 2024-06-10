package methods

import (
	"context"
	"encoding/hex"

	"github.com/creachadair/jrpc2"
	"github.com/HashCash-Consultants/go/network"
	proto "github.com/HashCash-Consultants/go/protocols/hcnetcore"
	"github.com/HashCash-Consultants/go/support/log"
	"github.com/HashCash-Consultants/go/xdr"

	"github.com/HashCash-Consultants/soroban-rpc/cmd/soroban-rpc/internal/daemon/interfaces"
)

// SendTransactionResponse represents the transaction submission response returned Soroban-RPC
type SendTransactionResponse struct {
	// ErrorResultXDR is present only if Status is equal to proto.TXStatusError.
	// ErrorResultXDR is a TransactionResult xdr string which contains details on why
	// the transaction could not be accepted by hcnet-core.
	ErrorResultXDR string `json:"errorResultXdr,omitempty"`
	// DiagnosticEventsXDR is present only if Status is equal to proto.TXStatusError.
	// DiagnosticEventsXDR is a base64-encoded slice of xdr.DiagnosticEvent
	DiagnosticEventsXDR []string `json:"diagnosticEventsXdr,omitempty"`
	// Status represents the status of the transaction submission returned by hcnet-core.
	// Status can be one of: proto.TXStatusPending, proto.TXStatusDuplicate,
	// proto.TXStatusTryAgainLater, or proto.TXStatusError.
	Status string `json:"status"`
	// Hash is a hash of the transaction which can be used to look up whether
	// the transaction was included in the ledger.
	Hash string `json:"hash"`
	// LatestLedger is the latest ledger known to Soroban-RPC at the time it handled
	// the transaction submission request.
	LatestLedger uint32 `json:"latestLedger"`
	// LatestLedgerCloseTime is the unix timestamp of the close time of the latest ledger known to
	// Soroban-RPC at the time it handled the transaction submission request.
	LatestLedgerCloseTime int64 `json:"latestLedgerCloseTime,string"`
}

// SendTransactionRequest is the Soroban-RPC request to submit a transaction.
type SendTransactionRequest struct {
	// Transaction is the base64 encoded transaction envelope.
	Transaction string `json:"transaction"`
}

// NewSendTransactionHandler returns a submit transaction json rpc handler
func NewSendTransactionHandler(daemon interfaces.Daemon, logger *log.Entry, ledgerRangeGetter LedgerRangeGetter, passphrase string) jrpc2.Handler {
	submitter := daemon.CoreClient()
	return NewHandler(func(ctx context.Context, request SendTransactionRequest) (SendTransactionResponse, error) {
		var envelope xdr.TransactionEnvelope
		err := xdr.SafeUnmarshalBase64(request.Transaction, &envelope)
		if err != nil {
			return SendTransactionResponse{}, &jrpc2.Error{
				Code:    jrpc2.InvalidParams,
				Message: "invalid_xdr",
			}
		}

		var hash [32]byte
		hash, err = network.HashTransactionInEnvelope(envelope, passphrase)
		if err != nil {
			return SendTransactionResponse{}, &jrpc2.Error{
				Code:    jrpc2.InvalidParams,
				Message: "invalid_hash",
			}
		}
		txHash := hex.EncodeToString(hash[:])

		latestLedgerInfo := ledgerRangeGetter.GetLedgerRange().LastLedger
		resp, err := submitter.SubmitTransaction(ctx, request.Transaction)
		if err != nil {
			logger.WithError(err).
				WithField("tx", request.Transaction).Error("could not submit transaction")
			return SendTransactionResponse{}, &jrpc2.Error{
				Code:    jrpc2.InternalError,
				Message: "could not submit transaction to hcnet-core",
			}
		}

		// interpret response
		if resp.IsException() {
			logger.WithField("exception", resp.Exception).
				WithField("tx", request.Transaction).Error("received exception from hcnet core")
			return SendTransactionResponse{}, &jrpc2.Error{
				Code:    jrpc2.InternalError,
				Message: "received exception from hcnet-core",
			}
		}

		switch resp.Status {
		case proto.TXStatusError:
			events, err := proto.DiagnosticEventsToSlice(resp.DiagnosticEvents)
			if err != nil {
				logger.WithField("tx", request.Transaction).Error("Cannot decode diagnostic events:", err)
				return SendTransactionResponse{}, &jrpc2.Error{
					Code:    jrpc2.InternalError,
					Message: "could not decode diagnostic events",
				}
			}
			return SendTransactionResponse{
				ErrorResultXDR:        resp.Error,
				DiagnosticEventsXDR:   events,
				Status:                resp.Status,
				Hash:                  txHash,
				LatestLedger:          latestLedgerInfo.Sequence,
				LatestLedgerCloseTime: latestLedgerInfo.CloseTime,
			}, nil
		case proto.TXStatusPending, proto.TXStatusDuplicate, proto.TXStatusTryAgainLater:
			return SendTransactionResponse{
				Status:                resp.Status,
				Hash:                  txHash,
				LatestLedger:          latestLedgerInfo.Sequence,
				LatestLedgerCloseTime: latestLedgerInfo.CloseTime,
			}, nil
		default:
			logger.WithField("status", resp.Status).
				WithField("tx", request.Transaction).Error("Unrecognized hcnet-core status response")
			return SendTransactionResponse{}, &jrpc2.Error{
				Code:    jrpc2.InternalError,
				Message: "invalid status from hcnet-core",
			}
		}
	})
}
