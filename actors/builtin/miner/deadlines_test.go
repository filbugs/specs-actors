package miner_test

import (
	"testing"

	"github.com/filecoin-project/go-bitfield"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/specs-actors/actors/abi"
	"github.com/filecoin-project/specs-actors/actors/builtin/miner"
)

const partSize = uint64(1000)

func TestProvingPeriodDeadlines(t *testing.T) {
	PP := miner.WPoStProvingPeriod
	CW := miner.WPoStChallengeWindow
	DLS := miner.WPoStPeriodDeadlines

	t.Run("pre-open", func(t *testing.T) {
		curr := abi.ChainEpoch(0) // Current is before the period opens.
		{
			periodStart := miner.FaultDeclarationCutoff + 1
			di := miner.ComputeProvingPeriodDeadline(periodStart, curr)
			assert.Equal(t, uint64(0), di.Index)
			assert.Equal(t, periodStart, di.Open)

			assert.False(t, di.PeriodStarted())
			assert.False(t, di.IsOpen())
			assert.False(t, di.HasElapsed())
			assert.False(t, di.FaultCutoffPassed())
			assert.Equal(t, periodStart+miner.WPoStProvingPeriod-1, di.PeriodEnd())
			assert.Equal(t, periodStart+miner.WPoStProvingPeriod, di.NextPeriodStart())
		}
		{
			periodStart := miner.FaultDeclarationCutoff - 1
			di := miner.ComputeProvingPeriodDeadline(periodStart, curr)
			assert.True(t, di.FaultCutoffPassed())
		}
	})

	t.Run("offset zero", func(t *testing.T) {
		firstPeriodStart := abi.ChainEpoch(0)

		// First proving period.
		di := assertDeadlineInfo(t, 0, firstPeriodStart, 0, 0)
		assert.Equal(t, -miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, -miner.FaultDeclarationCutoff, di.FaultCutoff)
		assert.True(t, di.IsOpen())
		assert.True(t, di.FaultCutoffPassed())

		assertDeadlineInfo(t, 1, firstPeriodStart, 0, 0)
		// Final epoch of deadline 0.
		assertDeadlineInfo(t, CW-1, firstPeriodStart, 0, 0)
		// First epoch of deadline 1
		assertDeadlineInfo(t, CW, firstPeriodStart, 1, CW)
		assertDeadlineInfo(t, CW+1, firstPeriodStart, 1, CW)
		// Final epoch of deadline 1
		assertDeadlineInfo(t, CW*2-1, firstPeriodStart, 1, CW)
		// First epoch of deadline 2
		assertDeadlineInfo(t, CW*2, firstPeriodStart, 2, CW*2)

		// Last epoch of last deadline
		assertDeadlineInfo(t, PP-1, firstPeriodStart, DLS-1, PP-CW)

		// Second proving period
		// First epoch of deadline 0
		secondPeriodStart := PP
		di = assertDeadlineInfo(t, PP, secondPeriodStart, 0, PP)
		assert.Equal(t, PP-miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, PP-miner.FaultDeclarationCutoff, di.FaultCutoff)

		// Final epoch of deadline 0.
		assertDeadlineInfo(t, PP+CW-1, secondPeriodStart, 0, PP+0)
		// First epoch of deadline 1
		assertDeadlineInfo(t, PP+CW, secondPeriodStart, 1, PP+CW)
		assertDeadlineInfo(t, PP+CW+1, secondPeriodStart, 1, PP+CW)
	})

	t.Run("offset non-zero", func(t *testing.T) {
		offset := CW*2 + 2 // Arbitrary not aligned with challenge window.
		initialPPStart := offset - PP
		firstDlIndex := miner.WPoStPeriodDeadlines - uint64(offset/CW) - 1
		firstDlOpen := initialPPStart + CW*abi.ChainEpoch(firstDlIndex)

		require.True(t, offset < PP)
		require.True(t, initialPPStart < 0)
		require.True(t, firstDlOpen < 0)

		// Incomplete initial proving period.
		// At epoch zero, the initial deadlines in the period have already passed and we're part way through
		// another one.
		di := assertDeadlineInfo(t, 0, initialPPStart, firstDlIndex, firstDlOpen)
		assert.Equal(t, firstDlOpen-miner.WPoStChallengeLookback, di.Challenge)
		assert.Equal(t, firstDlOpen-miner.FaultDeclarationCutoff, di.FaultCutoff)
		assert.True(t, di.IsOpen())
		assert.True(t, di.FaultCutoffPassed())

		// Epoch 1
		assertDeadlineInfo(t, 1, initialPPStart, firstDlIndex, firstDlOpen)

		// Epoch 2 rolls over to third-last challenge window
		assertDeadlineInfo(t, 2, initialPPStart, firstDlIndex+1, firstDlOpen+CW)
		assertDeadlineInfo(t, 3, initialPPStart, firstDlIndex+1, firstDlOpen+CW)

		// Last epoch of second-last window.
		assertDeadlineInfo(t, 2+CW-1, initialPPStart, firstDlIndex+1, firstDlOpen+CW)
		// First epoch of last challenge window.
		assertDeadlineInfo(t, 2+CW, initialPPStart, firstDlIndex+2, firstDlOpen+CW*2)
		// Last epoch of last challenge window.
		assert.Equal(t, miner.WPoStPeriodDeadlines-1, firstDlIndex+2)
		assertDeadlineInfo(t, 2+2*CW-1, initialPPStart, firstDlIndex+2, firstDlOpen+CW*2)

		// First epoch of next proving period.
		assertDeadlineInfo(t, 2+2*CW, initialPPStart+PP, 0, initialPPStart+PP)
		assertDeadlineInfo(t, 2+2*CW+1, initialPPStart+PP, 0, initialPPStart+PP)
	})

	t.Run("period expired", func(t *testing.T) {
		offset := abi.ChainEpoch(1)
		d := miner.ComputeProvingPeriodDeadline(offset, offset+miner.WPoStProvingPeriod)
		assert.True(t, d.PeriodStarted())
		assert.True(t, d.PeriodElapsed())
		assert.Equal(t, miner.WPoStPeriodDeadlines, d.Index)
		assert.False(t, d.IsOpen())
		assert.True(t, d.HasElapsed())
		assert.True(t, d.FaultCutoffPassed())
		assert.Equal(t, offset+miner.WPoStProvingPeriod-1, d.PeriodEnd())
		assert.Equal(t, offset+miner.WPoStProvingPeriod, d.NextPeriodStart())
	})
}

func assertDeadlineInfo(t *testing.T, current, periodStart abi.ChainEpoch, expectedIndex uint64, expectedDeadlineOpen abi.ChainEpoch) *miner.DeadlineInfo {
	expected := makeDeadline(current, periodStart, expectedIndex, expectedDeadlineOpen)
	actual := miner.ComputeProvingPeriodDeadline(periodStart, current)
	assert.True(t, actual.PeriodStarted())
	assert.True(t, actual.IsOpen())
	assert.False(t, actual.HasElapsed())
	assert.Equal(t, expected, actual)
	return actual
}

func makeDeadline(currEpoch, periodStart abi.ChainEpoch, index uint64, deadlineOpen abi.ChainEpoch) *miner.DeadlineInfo {
	return &miner.DeadlineInfo{
		CurrentEpoch: currEpoch,
		PeriodStart:  periodStart,
		Index:        index,
		Open:         deadlineOpen,
		Close:        deadlineOpen + miner.WPoStChallengeWindow,
		Challenge:    deadlineOpen - miner.WPoStChallengeLookback,
		FaultCutoff:  deadlineOpen - miner.FaultDeclarationCutoff,
	}
}

func TestPartitionsForDeadline(t *testing.T) {
	t.Run("empty deadlines", func(t *testing.T) {
		dl := deadlineWithSectors(t, [miner.WPoStPeriodDeadlines]uint64{})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})

	t.Run("single sector at first deadline", func(t *testing.T) {
		dl := deadlineWithSectors(t, [miner.WPoStPeriodDeadlines]uint64{1})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(1), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Zero(t, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Zero(t, sectorCount)
	})

	t.Run("single sector at non-first deadline", func(t *testing.T) {
		dl := deadlineWithSectors(t, [miner.WPoStPeriodDeadlines]uint64{0, 1})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, uint64(1), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})

	t.Run("deadlines with one full partitions", func(t *testing.T) {
		dl := deadlinesWithFullPartitions(t, 1)
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, miner.WPoStPeriodDeadlines-1, firstIndex)
		assert.Equal(t, partSize, sectorCount)
	})

	t.Run("partial partitions", func(t *testing.T) {
		dl := deadlineWithSectors(t, [miner.WPoStPeriodDeadlines]uint64{
			0: partSize - 1,
			1: partSize,
			2: partSize - 2,
			3: partSize,
			4: partSize - 3,
			5: partSize,
		})
		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(2), firstIndex)
		assert.Equal(t, partSize-2, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 5)
		require.NoError(t, err)
		assert.Equal(t, uint64(5), firstIndex)
		assert.Equal(t, partSize, sectorCount)
	})

	t.Run("multiple partitions", func(t *testing.T) {
		dl := deadlineWithSectors(t, [miner.WPoStPeriodDeadlines]uint64{
			0: partSize,       // 1 partition 1 total
			1: partSize * 2,   // 2 partitions 3 total
			2: partSize*4 - 1, // 4 partitions 7 total
			3: partSize * 6,   // 6 partitions 13 total
			4: partSize*8 - 1, // 8 partitions 21 total
			5: partSize * 9,   // 9 partitions 30 total
		})

		firstIndex, sectorCount, err := miner.PartitionsForDeadline(dl, partSize, 0)
		require.NoError(t, err)
		assert.Equal(t, uint64(0), firstIndex)
		assert.Equal(t, partSize, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 1)
		require.NoError(t, err)
		assert.Equal(t, uint64(1), firstIndex)
		assert.Equal(t, partSize*2, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 2)
		require.NoError(t, err)
		assert.Equal(t, uint64(3), firstIndex)
		assert.Equal(t, partSize*4-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 3)
		require.NoError(t, err)
		assert.Equal(t, uint64(7), firstIndex)
		assert.Equal(t, partSize*6, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 4)
		require.NoError(t, err)
		assert.Equal(t, uint64(13), firstIndex)
		assert.Equal(t, partSize*8-1, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, 5)
		require.NoError(t, err)
		assert.Equal(t, uint64(21), firstIndex)
		assert.Equal(t, partSize*9, sectorCount)

		firstIndex, sectorCount, err = miner.PartitionsForDeadline(dl, partSize, miner.WPoStPeriodDeadlines-1)
		require.NoError(t, err)
		assert.Equal(t, uint64(30), firstIndex)
		assert.Equal(t, uint64(0), sectorCount)
	})
}

func TestComputePartitionsSectors(t *testing.T) {
	t.Run("no partitions due at empty deadline", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bf(0, 1)

		// No partitions at deadline 0
		_, err := miner.ComputePartitionsSectors(dls, partSize, 0, []uint64{0})
		require.Error(t, err)

		// No partitions at deadline 2
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{1})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{2})
		require.Error(t, err)
	})
	t.Run("single sector", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bf(0, 1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bf(0, 1), partitions[0])
	})
	t.Run("full partition", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[10] = bf(1234, partSize)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bf(1234, partSize), partitions[0])
	})
	t.Run("full plus partial partition", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[10] = bf(5555, partSize+1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0}) // First partition
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bf(5555, partSize), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{1}) // Second partition
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bf(5555+partSize, 1), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 10, []uint64{0, 1}) // Both partitions
		require.NoError(t, err)
		assert.Equal(t, 2, len(partitions))
		assertBfEqual(t, bf(5555, partSize), partitions[0])
		assertBfEqual(t, bf(5555+partSize, 1), partitions[1])
	})
	t.Run("multiple partitions", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bf(0, 3*partSize+1)
		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0, 1, 2, 3})
		require.NoError(t, err)
		assert.Equal(t, 4, len(partitions))
		assertBfEqual(t, bf(0, partSize), partitions[0])
		assertBfEqual(t, bf(1*partSize, partSize), partitions[1])
		assertBfEqual(t, bf(2*partSize, partSize), partitions[2])
		assertBfEqual(t, bf(3*partSize, 1), partitions[3])
	})
	t.Run("partitions numbered across deadlines", func(t *testing.T) {
		dls := miner.ConstructDeadlines()
		dls.Due[1] = bf(0, 3*partSize+1)
		dls.Due[3] = bf(3*partSize+1, 1)
		dls.Due[5] = bf(3*partSize+1+1, 2*partSize)

		partitions, err := miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{0, 1, 2, 3})
		require.NoError(t, err)
		assert.Equal(t, 4, len(partitions))

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{4})
		require.NoError(t, err)
		assert.Equal(t, 1, len(partitions))
		assertBfEqual(t, bf(3*partSize+1, 1), partitions[0])

		partitions, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{5, 6})
		require.NoError(t, err)
		assert.Equal(t, 2, len(partitions))
		assertBfEqual(t, bf(3*partSize+1+1, partSize), partitions[0])
		assertBfEqual(t, bf(3*partSize+1+1+partSize, partSize), partitions[1])

		// Mismatched deadline/partition pairs
		_, err = miner.ComputePartitionsSectors(dls, partSize, 1, []uint64{4})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 2, []uint64{4})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{3})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 3, []uint64{5})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 4, []uint64{5})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{0})
		require.Error(t, err)
		_, err = miner.ComputePartitionsSectors(dls, partSize, 5, []uint64{7})
		require.Error(t, err)
	})
}

//
// Deadlines Utils
//

func assertBfEqual(t *testing.T, expected, actual *bitfield.BitField) {
	ex, err := expected.All(1 << 20)
	require.NoError(t, err)
	ac, err := actual.All(1 << 20)
	require.NoError(t, err)
	assert.Equal(t, ex, ac)
}

// Creates a bitfield with a contiguous run of `count` values from `first.
func bf(first uint64, count uint64) *abi.BitField {
	values := make([]uint64, count)
	for i := range values {
		values[i] = first + uint64(i)
	}
	return bitfield.NewFromSet(values)
}

func deadlinesWithFullPartitions(t *testing.T, n uint64) *miner.Deadlines {
	gen := [miner.WPoStPeriodDeadlines]uint64{}
	for i := range gen {
		gen[i] = partSize * n
	}
	return deadlineWithSectors(t, gen)
}

// accepts an array were the value at each index indicates how many sectors are in the partition of the returned Deadlines
// Example:
// gen := [miner.WPoStPeriodDeadlines]uint64{1, 42, 89, 0} returns a deadline with:
// 1  sectors at deadlineIdx 0
// 42 sectors at deadlineIdx 1
// 89 sectors at deadlineIdx 2
// 0  sectors at deadlineIdx 3-47
func deadlineWithSectors(t *testing.T, gen [miner.WPoStPeriodDeadlines]uint64) *miner.Deadlines {
	// ensure there are no duplicate sectors across partitions
	var sectorIdx uint64
	dls := miner.ConstructDeadlines()
	for partition, numSectors := range gen {
		var sectors []uint64
		for i := uint64(0); i < numSectors; i++ {
			sectors = append(sectors, sectorIdx)
			sectorIdx++
		}
		require.NoError(t, dls.AddToDeadline(uint64(partition), sectors...))
	}
	return dls
}
