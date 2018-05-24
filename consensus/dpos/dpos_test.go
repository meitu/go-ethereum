package dpos

import (
	"math/big"
	"testing"

	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/assert"
)

var (
	MockEpoch = []string{
		"0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e",
		"0xa60a3886b552ff9992cfcd208ec1152079e046c2",
		"0x4e080e49f62694554871e669aeb4ebe17c4a9670",
		"0xb040353ec0f2c113d5639444f7253681aecda1f8",
		"0x14432e15f21237013017fa6ee90fc99433dec82c",
		"0x9f30d0e5c9c88cade54cd1adecf6bc2c7e0e5af6",
		"0xd83b44a3719720ec54cdb9f54c0202de68f1ebcb",
		"0x56cc452e450551b7b9cffe25084a069e8c1e9441",
		"0xbcfcb3fa8250be4f2bf2b1e70e1da500c668377b",
		"0x9d9667c71bb09d6ca7c3ed12bfe5e7be24e2ffe1",
		"0xabde197e97398864ba74511f02832726edad5967",
		"0x6f99d97a394fa7a623fdf84fdc7446b99c3cb335",
		"0xf78b011e639ce6d8b76f97712118f3fe4a12dd95",
		"0x8db3b6c801dddd624d6ddc2088aa64b5a2493661",
		"0x751b484bd5296f8d267a8537d33f25a848f7f7af",
	}
)

func mockNewDposContext(db ethdb.Database) *types.DposContext {
	dposContext, err := types.NewDposContextFromProto(db, &types.DposContextProto{})
	if err != nil {
		return nil
	}
	delegator := []byte{}
	candidate := []byte{}
	addresses := []common.Address{}
	for i := 0; i < epochSize; i++ {
		addresses = append(addresses, common.HexToAddress(MockEpoch[i]))
	}
	dposContext.SetValidators(addresses)
	for j := 0; j < len(MockEpoch); j++ {
		delegator = common.HexToAddress(MockEpoch[j]).Bytes()
		candidate = common.HexToAddress(MockEpoch[j]).Bytes()
		dposContext.DelegateTrie().TryUpdate(append(candidate, delegator...), candidate)
		dposContext.CandidateTrie().TryUpdate(candidate, candidate)
		dposContext.VoteTrie().TryUpdate(candidate, candidate)
	}
	return dposContext
}

func mockNewBlock(root common.Hash, db *ethdb.MemDatabase, miner string, time int64) *types.Block {
	dposContext := mockNewDposContext(db)
	header := &types.Header{
		ParentHash:  common.Hash{},
		UncleHash:   common.Hash{},
		Coinbase:    common.HexToAddress(miner),
		Root:        root,
		TxHash:      common.Hash{},
		ReceiptHash: common.Hash{},
		DposContext: dposContext.ToProto(),
		Bloom:       types.Bloom{},
		Difficulty:  big.NewInt(0),
		Number:      big.NewInt(0),
		GasLimit:    big.NewInt(0),
		GasUsed:     big.NewInt(0),
		Time:        big.NewInt(time),
		Extra:       []byte{},
		MixDigest:   common.Hash{},
		Nonce:       types.BlockNonce{},
	}
	block := types.NewBlock(header, nil, nil, nil)
	block.DposContext = dposContext
	return block
}

func mockNewEpochContext(time int64, validator common.Address) *EpochContext {
	db, _ := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	epochContext := &EpochContext{
		time,
		mockNewDposContext(db),
		stateDB,
	}
	return epochContext
}

func setMintCntTrie(epochID int64, candidate common.Address, mintCntTrie *trie.Trie, count int64) {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(cntBytes, uint64(count))
	mintCntTrie.TryUpdate(append(key, candidate.Bytes()...), cntBytes)
}

func getMintCnt(epochID int64, candidate common.Address, mintCntTrie *trie.Trie) int64 {
	key := make([]byte, 8)
	binary.BigEndian.PutUint64(key, uint64(epochID))
	cntBytes := mintCntTrie.Get(append(key, candidate.Bytes()...))
	if cntBytes == nil {
		return 0
	} else {
		return int64(binary.BigEndian.Uint64(cntBytes))
	}
}

func TestTryElect(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))

	addresses := []common.Address{}
	for i := 0; i < len(MockEpoch); i++ {
		addr := common.StringToAddress(MockEpoch[i])
		addresses = append(addresses, addr)
		bal := int64(1000 * (i + 1))
		stateDB.CreateAccount(addr)
		stateDB.SetBalance(addr, big.NewInt(bal))
	}
	root := stateDB.IntermediateRoot(false)

	genesis := mockNewBlock(root, db, "0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6c", 0)
	parent := mockNewBlock(root, db, "0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e", 3600)

	// forge 3 candidates mint count（only MockEpoch[1] not satisfy: cnt>=epochInterval/blockInterval/epochSize/2=120 ）
	setMintCntTrie(parent.Time().Int64()/epochInterval, common.HexToAddress(MockEpoch[0]), parent.DposContext.MintCntTrie(), int64(256))
	setMintCntTrie(parent.Time().Int64()/epochInterval, common.HexToAddress(MockEpoch[1]), parent.DposContext.MintCntTrie(), int64(100))
	setMintCntTrie(parent.Time().Int64()/epochInterval, common.HexToAddress(MockEpoch[2]), parent.DposContext.MintCntTrie(), int64(360))

	dposContextBeforeElection := parent.DposContext
	dposContextAfterElection := dposContextBeforeElection
	assert.NotEqual(t, dposContextBeforeElection, nil)

	epochContext := &EpochContext{
		statedb:     stateDB,
		DposContext: dposContextAfterElection,
		TimeStamp:   3700,
	}
	// now(=3700) is still in the same epoch with parent,
	// preDposContext will be not changed
	epochContext.tryElect(genesis.Header(), parent.Header())
	assert.NotEqual(t, dposContextAfterElection, nil)

	// and MockEpoch[1] is still in DposContext
	assert.NotEqual(t, nil, dposContextAfterElection.CandidateTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))
	assert.NotEqual(t, nil, dposContextAfterElection.DelegateTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))
	assert.NotEqual(t, nil, dposContextAfterElection.VoteTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))

	// now(=7200) comes to a new epoch ,
	// preDposContext will be changed
	epochContext.TimeStamp = 7200
	epochContext.tryElect(genesis.Header(), parent.Header())
	assert.NotEqual(t, dposContextAfterElection, nil)
	// todo: safeSize = 3 no candidate can be kickout
	//  MockEpoch[1] is kickout from dposContext
	//assert.Equal(t, []byte(nil), dposContextAfterElection.CandidateTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))
	//assert.Equal(t, []byte(nil), dposContextAfterElection.DelegateTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))
	//assert.Equal(t, []byte(nil), dposContextAfterElection.VoteTrie().Get(common.HexToAddress(MockEpoch[1]).Bytes()))
}

func TestUpdateMintCnt(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()
	dposContext := mockNewDposContext(db)

	// new block still in the same epoch with current block, but newMiner is the first time to mint in the epoch
	lastTime := int64(3600)

	miner := common.HexToAddress("0xa60a3886b552ff9992cfcd208ec1152079e046c2")
	blockTime := int64(3605)

	beforeUpdateCnt := getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt := getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)

	// new block still in the same epoch with current block, and newMiner has mint block before in the epoch
	setMintCntTrie(blockTime/epochInterval, miner, dposContext.MintCntTrie(), int64(1))

	blockTime = 3620

	// currentBlock has recorded the count for the newMiner before UpdateMintCnt
	beforeUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(1), beforeUpdateCnt)
	assert.Equal(t, int64(2), afterUpdateCnt)

	// new block come to a new epoch
	blockTime = 7200

	beforeUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	updateMintCnt(lastTime, blockTime, miner, dposContext)
	afterUpdateCnt = getMintCnt(blockTime/epochInterval, miner, dposContext.MintCntTrie())
	assert.Equal(t, int64(0), beforeUpdateCnt)
	assert.Equal(t, int64(1), afterUpdateCnt)
}

func TestKickoutCandidate(t *testing.T) {
	validator := common.HexToAddress("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e")
	time := int64(3600)
	epoch := time / epochInterval
	epochContext := mockNewEpochContext(time, validator)

	// forge 3 candidates mint count（none satisfy: cnt>=epochInterval/blockInterval/epochSize/2 ）
	for i := 0; i < epochSize; i++ {
		setMintCntTrie(epoch, common.HexToAddress(MockEpoch[i]), epochContext.DposContext.MintCntTrie(), int64(i+1))
	}

	candidateTrie := epochContext.DposContext.CandidateTrie()
	delegateTrie := epochContext.DposContext.DelegateTrie()
	voteTrie := epochContext.DposContext.VoteTrie()

	// before kickout, candidates from epochContext exist
	for i := 0; i < epochSize; i++ {
		assert.NotEqual(t, nil, candidateTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
		assert.NotEqual(t, nil, delegateTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
		assert.NotEqual(t, nil, voteTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
	}

	// kickout not active candidates
	assert.Equal(t, nil, epochContext.kickoutValidator(epoch))

	candidateTrie = epochContext.DposContext.CandidateTrie()
	delegateTrie = epochContext.DposContext.DelegateTrie()
	voteTrie = epochContext.DposContext.VoteTrie()

	// candidates of epochTrie will be kickout
	for i := 0; i < epochSize-safeSize; i++ {
		assert.Equal(t, []byte(nil), candidateTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
		assert.Equal(t, []byte(nil), delegateTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
		assert.Equal(t, []byte(nil), voteTrie.Get(common.HexToAddress(MockEpoch[i]).Bytes()))
	}
}
