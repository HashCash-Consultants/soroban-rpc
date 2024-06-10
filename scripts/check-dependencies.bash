#!/bin/bash

set -e

SED=sed
if [ -z "$(sed --version 2>&1 | grep GNU)" ]; then
    SED=gsed
fi

CURL="curl -sL --fail-with-body"

if ! CARGO_OUTPUT=$(cargo tree -p soroban-env-host 2>&1); then
  echo "The project depends on multiple versions of the soroban-env-host Rust library, please unify them."
  echo "Make sure the soroban-sdk dependency indirectly points to the same soroban-env-host dependency imported explicitly."
  echo
  echo "This is soroban-env-host version imported by soroban-sdk:"
  cargo tree --depth 1  -p soroban-sdk | grep env-host
  echo
  echo
  echo
  echo "Full error:"
  echo $CARGO_OUTPUT
  exit 1
fi


# revision of the https://github.com/HashCash-Consultants/rs-hcnet-xdr library used by the Rust code
RS_HCNET_XDR_REVISION=""

# revision of https://github.com/HashCash-Consultants/hcnet-xdr/ used by the Rust code
HCNET_XDR_REVISION_FROM_RUST=""

function hcnet_xdr_version_from_rust_dep_tree {
  LINE=$(grep hcnet-xdr | head -n 1)
  # try to obtain a commit
  COMMIT=$(echo $LINE | $SED -n 's/.*rev=\(.*\)#.*/\1/p')
  if [ -n "$COMMIT" ]; then
    echo "$COMMIT"
    return
  fi
  # obtain a crate version
  echo $LINE | $SED -n  's/.*hcnet-xdr \(v\)\{0,1\}\([^ ]*\).*/\2/p'
}

if CARGO_OUTPUT=$(cargo tree --depth 0 -p hcnet-xdr 2>&1); then
  RS_HCNET_XDR_REVISION=$(echo "$CARGO_OUTPUT" | hcnet_xdr_version_from_rust_dep_tree)
  if [ ${#RS_HCNET_XDR_REVISION} -eq 40 ]; then
    # revision is a git hash
    HCNET_XDR_REVISION_FROM_RUST=$($CURL https://raw.githubusercontent.com/hcnet/rs-hcnet-xdr/${RS_HCNET_XDR_REVISION}/xdr/curr-version)
  else
    # revision is a crate version
    CARGO_SRC_BASE_DIR=$(realpath ${CARGO_HOME:-$HOME/.cargo}/registry/src/index*)
    HCNET_XDR_REVISION_FROM_RUST=$(cat "${CARGO_SRC_BASE_DIR}/hcnet-xdr-${RS_HCNET_XDR_REVISION}/xdr/curr-version")
  fi
else
  echo "The project depends on multiple versions of the Rust rs-hcnet-xdr library"
  echo "Make sure a single version of hcnet-xdr is used"
  echo
  echo
  echo
  echo "Full error:"
  echo $CARGO_OUTPUT
fi

# Now, lets compare the Rust and Go XDR revisions
# TODO: The sed extraction below won't work for version tags
GO_XDR_REVISION=$(go list -m -f '{{.Version}}' github.com/HashCash-Consultants/go | $SED 's/.*-\(.*\)/\1/')

# revision of https://github.com/HashCash-Consultants/hcnet-xdr/ used by the Go code
HCNET_XDR_REVISION_FROM_GO=$($CURL https://raw.githubusercontent.com/hcnet/go/${GO_XDR_REVISION}/xdr/xdr_commit_generated.txt)

if [ "$HCNET_XDR_REVISION_FROM_GO" != "$HCNET_XDR_REVISION_FROM_RUST" ]; then
  echo "Go and Rust dependencies are using different revisions of https://github.com/HashCash-Consultants/hcnet-xdr"
  echo
  echo "Rust dependencies are using commit $HCNET_XDR_REVISION_FROM_RUST"
  echo "Go dependencies are using commit $HCNET_XDR_REVISION_FROM_GO"
  exit 1
fi

# Now, lets make sure that the core and captive core version used in the tests use the same version and that they depend
# on the same XDR revision

PROTOCOL_VERSIONS=$(grep 'protocol-version: ' .github/workflows/soroban-rpc.yml | grep -o -E '[0-9]+')

for P in $PROTOCOL_VERSIONS; do
    CORE_CONTAINER_REVISION=$($SED -n 's/.*PROTOCOL_'$P'_CORE_DOCKER_IMG:.*\/\(hcnet-core\|unsafe-hcnet-core\(-next\)\{0,1\}\)\:.*\.\([a-zA-Z0-9]*\)\..*/\3/p' < .github/workflows/soroban-rpc.yml)
    CAPTIVE_CORE_PKG_REVISION=$($SED -n 's/.*PROTOCOL_'$P'_CORE_DEBIAN_PKG_VERSION:.*\.\([a-zA-Z0-9]*\)\..*/\1/p' < .github/workflows/soroban-rpc.yml)

    if [ "$CORE_CONTAINER_REVISION" != "$CAPTIVE_CORE_PKG_REVISION" ]; then
	    echo "Soroban RPC protocol $P integration tests are using different versions of the Core container and Captive Core Debian package."
	    echo
	    echo "Core container image commit $CORE_CONTAINER_REVISION"
	    echo "Captive core debian package commit $CAPTIVE_CORE_PKG_REVISION"
	    exit 1
    fi

    # Revision of https://github.com/HashCash-Consultants/rs-hcnet-xdr by Core.
    # We obtain it from src/rust/src/host-dep-tree-curr.txt but Alternatively/in addition we could:
    #  * Check the rs-hcnet-xdr revision of host-dep-tree-prev.txt
    #  * Check the hcnet-xdr revision
    CORE_HOST_DEP_TREE_CURR=$($CURL https://raw.githubusercontent.com/hcnet/hcnet-core/${CORE_CONTAINER_REVISION}/src/rust/src/host-dep-tree-curr.txt)


    RS_HCNET_XDR_REVISION_FROM_CORE=$(echo "$CORE_HOST_DEP_TREE_CURR" | hcnet_xdr_version_from_rust_dep_tree)
    if [ "$RS_HCNET_XDR_REVISION" != "$RS_HCNET_XDR_REVISION_FROM_CORE" ]; then
	    echo "The Core revision used in protocol $P integration tests (${CORE_CONTAINER_REVISION}) uses a different revision of https://github.com/HashCash-Consultants/rs-hcnet-xdr"
	    echo
	    echo "Current repository's revision $RS_HCNET_XDR_REVISION"
	    echo "Core's revision $RS_HCNET_XDR_REVISION_FROM_CORE"
	    # Temporarily skip check since Core is not currently using the exact version rs-hcnet-xdr revision
	    # exit 1
    fi
done



