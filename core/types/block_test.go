// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/rlp"
)

// from bcValidBlockTest.json, "SimpleTx"
func TestBlockEncoding(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()
	dposCtx, _ := NewDposContext(db)
	inputBlock := Block{
		header: &Header{
			Difficulty:  big.NewInt(131072),
			GasLimit:    big.NewInt(3141592),
			GasUsed:     big.NewInt(21000),
			Validator:   common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
			Coinbase:    common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"),
			MixDigest:   common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"),
			Root:        common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"),
			Nonce:       EncodeNonce(uint64(0xa13a5a8c8f2bb1c4)),
			Time:        big.NewInt(1426516743),
			DposContext: dposCtx.ToProto(),
		},
	}
	tx1 := NewTransaction(Binary, 0, common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"), big.NewInt(10), big.NewInt(50000), big.NewInt(10), nil)
	tx1, _ = tx1.WithSignature(HomesteadSigner{}, common.Hex2Bytes("9bea4c4daac7c7c52e093e6a4c35dbbcf8856f1af7b059ba20253e70848d094f8a8fae537ce25ed8cb5af9adac3f141af69bd515bd2ba031522df09b97dd72b100"))
	inputBlock.transactions = []*Transaction{tx1}
	inputHash := inputBlock.Hash()
	blockEnc, _ := rlp.EncodeToBytes(extblock{
		Header: inputBlock.header,
		Txs:    inputBlock.transactions,
		Uncles: inputBlock.uncles,
	})
	var block Block
	if err := rlp.DecodeBytes(blockEnc, &block); err != nil {
		t.Fatal("decode error: ", err)
	}

	check := func(f string, got, want interface{}) {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s mismatch: got %v, want %v", f, got, want)
		}
	}
	check("Difficulty", block.Difficulty(), big.NewInt(131072))
	check("GasLimit", block.GasLimit(), big.NewInt(3141592))
	check("GasUsed", block.GasUsed(), big.NewInt(21000))
	check("Validator", block.Validator(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("Coinbase", block.Coinbase(), common.HexToAddress("8888f1f195afa192cfee860698584c030f4c9db1"))
	check("MixDigest", block.MixDigest(), common.HexToHash("bd4472abb6659ebe3ee06ee4d7b72a00a9f4d001caca51342001075469aff498"))
	check("Root", block.Root(), common.HexToHash("ef1552a40b7165c3cd773806b9e0c165b75356e0314bf0706f279c729f51e017"))
	check("Nonce", block.Nonce(), uint64(0xa13a5a8c8f2bb1c4))
	check("Time", block.Time(), big.NewInt(1426516743))
	check("Size", block.Size(), common.StorageSize(len(blockEnc)))
	check("Hash", block.Hash(), inputHash)
	check("len(Transactions)", len(block.Transactions()), 1)
	check("Transactions[0].Hash", block.Transactions()[0].Hash(), tx1.Hash())
	ourBlockEnc, err := rlp.EncodeToBytes(&block)
	if err != nil {
		t.Fatal("encode error: ", err)
	}
	if !bytes.Equal(ourBlockEnc, blockEnc) {
		t.Errorf("encoded block mismatch:\ngot:  %x\nwant: %x", ourBlockEnc, blockEnc)
	}
}
