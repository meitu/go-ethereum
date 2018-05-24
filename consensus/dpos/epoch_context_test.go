package dpos

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/trie"

	"github.com/stretchr/testify/assert"
)

func TestEpochContextCountVotes(t *testing.T) {
	voteMap := map[common.Address][]common.Address{
		common.HexToAddress("0x44d1ce0b7cb3588bca96151fe1bc05af38f91b6e"): {
			common.HexToAddress("0xb040353ec0f2c113d5639444f7253681aecda1f8"),
		},
		common.HexToAddress("0xa60a3886b552ff9992cfcd208ec1152079e046c2"): {
			common.HexToAddress("0x14432e15f21237013017fa6ee90fc99433dec82c"),
			common.HexToAddress("0x9f30d0e5c9c88cade54cd1adecf6bc2c7e0e5af6"),
		},
		common.HexToAddress("0x4e080e49f62694554871e669aeb4ebe17c4a9670"): {
			common.HexToAddress("0xd83b44a3719720ec54cdb9f54c0202de68f1ebcb"),
			common.HexToAddress("0x56cc452e450551b7b9cffe25084a069e8c1e9441"),
			common.HexToAddress("0xbcfcb3fa8250be4f2bf2b1e70e1da500c668377b"),
		},
		common.HexToAddress("0x9d9667c71bb09d6ca7c3ed12bfe5e7be24e2ffe1"): {},
	}
	balance := int64(5)
	db, _ := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	dposContext, err := types.NewDposContext(db)
	assert.Nil(t, err)

	epochContext := &EpochContext{
		DposContext: dposContext,
		statedb:     stateDB,
	}
	_, err = epochContext.countVotes()
	assert.NotNil(t, err)

	for candidate, electors := range voteMap {
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
		for _, elector := range electors {
			stateDB.SetBalance(elector, big.NewInt(balance))
			assert.Nil(t, dposContext.Delegate(elector, candidate))
		}
	}
	result, err := epochContext.countVotes()
	assert.Nil(t, err)
	assert.Equal(t, len(voteMap), len(result))
	for candidate, electors := range voteMap {
		voteCount, ok := result[candidate]
		assert.True(t, ok)
		assert.Equal(t, balance*int64(len(electors)), voteCount.Int64())
	}
}

func TestLookupValidator(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()
	dposCtx, _ := types.NewDposContext(db)
	mockEpochContext := &EpochContext{
		DposContext: dposCtx,
	}
	validators := []common.Address{
		common.StringToAddress("addr1"),
		common.StringToAddress("addr2"),
		common.StringToAddress("addr3"),
	}
	mockEpochContext.DposContext.SetValidators(validators)
	for i, expected := range validators {
		got, _ := mockEpochContext.lookupValidator(int64(i) * blockInterval)
		if got != expected {
			t.Errorf("Failed to test lookup validator, %s was expected but got %s", expected.Str(), got.Str())
		}
	}
	_, err := mockEpochContext.lookupValidator(blockInterval - 1)
	if err != ErrInvalidMintBlockTime {
		t.Errorf("Failed to test lookup validator. err '%v' was expected but got '%v'", ErrInvalidMintBlockTime, err)
	}
}

func TestEpochContextKickoutValidator(t *testing.T) {
	db, _ := ethdb.NewMemDatabase()
	stateDB, _ := state.New(common.Hash{}, state.NewDatabase(db))
	dposContext, err := types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext := &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	atLeastMintCnt := epochInterval / blockInterval / epochSize / 2
	testEpoch := int64(1)

	// no validator can be kickout, because all validators mint enough block at least
	validators := []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, dposContext.BecomeCandidate(common.StringToAddress("addr")))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap := getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, epochSize+1, len(candidateMap))

	// atLeast a safeSize count candidate will reserve
	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-int64(i)-1)
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, safeSize, len(candidateMap))
	for i := epochSize - 1; i >= safeSize; i-- {
		assert.False(t, candidateMap[common.StringToAddress("addr"+strconv.Itoa(i))])
	}

	// all validator will be kickout, because all validators didn't mint enough block at least
	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
	}
	for i := epochSize; i < epochSize*2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, epochSize, len(candidateMap))

	// only one validator mint count is not enough
	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		if i == 0 {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt-1)
		} else {
			setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt)
		}
	}
	assert.Nil(t, dposContext.BecomeCandidate(common.StringToAddress("addr")))
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, epochSize, len(candidateMap))
	assert.False(t, candidateMap[common.StringToAddress("addr"+strconv.Itoa(0))])

	// epochTime is not complete, all validators mint enough block at least
	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2)
	}
	for i := epochSize; i < epochSize*2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, epochSize*2, len(candidateMap))

	// epochTime is not complete, all validators didn't mint enough block at least
	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	validators = []common.Address{}
	for i := 0; i < epochSize; i++ {
		validator := common.StringToAddress("addr" + strconv.Itoa(i))
		validators = append(validators, validator)
		assert.Nil(t, dposContext.BecomeCandidate(validator))
		setTestMintCnt(dposContext, testEpoch, validator, atLeastMintCnt/2-1)
	}
	for i := epochSize; i < epochSize*2; i++ {
		candidate := common.StringToAddress("addr" + strconv.Itoa(i))
		assert.Nil(t, dposContext.BecomeCandidate(candidate))
	}
	assert.Nil(t, dposContext.SetValidators(validators))
	assert.Nil(t, epochContext.kickoutValidator(testEpoch))
	candidateMap = getCandidates(dposContext.CandidateTrie())
	assert.Equal(t, epochSize, len(candidateMap))

	dposContext, err = types.NewDposContext(db)
	assert.Nil(t, err)
	epochContext = &EpochContext{
		TimeStamp:   epochInterval / 2,
		DposContext: dposContext,
		statedb:     stateDB,
	}
	assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
	dposContext.SetValidators([]common.Address{})
	assert.NotNil(t, epochContext.kickoutValidator(testEpoch))
}

func setTestMintCnt(dposContext *types.DposContext, epoch int64, validator common.Address, count int64) {
	for i := int64(0); i < count; i++ {
		updateMintCnt(epoch*epochInterval, epoch*epochInterval+blockInterval, validator, dposContext)
	}
}

func getCandidates(candidateTrie *trie.Trie) map[common.Address]bool {
	candidateMap := map[common.Address]bool{}
	iter := trie.NewIterator(candidateTrie.NodeIterator(nil))
	for iter.Next() {
		candidateMap[common.BytesToAddress(iter.Value)] = true
	}
	return candidateMap
}
