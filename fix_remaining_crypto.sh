#!/bin/bash

# Fix nodes.go
sed -i 's/trienode.New(crypto.Keccak256Hash(n.Blob), blob, n.Owner)/trienode.New(common.Hash(crypto.Keccak256Hash(n.Blob)), blob, n.Owner)/g' triedb/pathdb/nodes.go

# Fix reader.go - convert crypto.Hash to common.Hash for function calls
sed -i 's/dl.account(hash)/dl.account(common.Hash(hash))/g' triedb/pathdb/reader.go
sed -i 's/newAccountIdentQuery(hash)/newAccountIdentQuery(common.Hash(hash))/g' triedb/pathdb/reader.go
sed -i 's/dl.storage(addrHash, keyHash)/dl.storage(common.Hash(addrHash), common.Hash(keyHash))/g' triedb/pathdb/reader.go
sed -i 's/newStorageIdentQuery(addrHash, keyHash)/newStorageIdentQuery(common.Hash(addrHash), common.Hash(keyHash))/g' triedb/pathdb/reader.go

# Fix history_inspect.go - bidirectional conversion
sed -i 's/key := slot/key := crypto.Hash(slot)/g' triedb/pathdb/history_inspect.go
sed -i 's/\[key\]/[common.Hash(key)]/g' triedb/pathdb/history_inspect.go

# Fix bind/v2/lib.go
sed -i 's/return crypto.CreateAddress(opts.From, tx.Nonce())/return common.Address(crypto.CreateAddress(crypto.Address(opts.From), tx.Nonce()))/g' accounts/abi/bind/v2/lib.go

# Fix bind/v2/auth.go
sed -i 's/From: keyAddr/From: common.Address(keyAddr)/g' accounts/abi/bind/v2/auth.go

