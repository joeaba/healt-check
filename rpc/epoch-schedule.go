package rpc

import (
	"math"
	"math/bits"
)

type EpochSchedule struct {
	FirstNormalEpoch         Epoch  `json:"firstNormalEpoch"`
	FirstNormalSlot          Slot   `json:"firstNormalSlot"`
	LeaderScheduleSlotOffset uint64 `json:"leaderScheduleSlotOffset"`
	SlotsPerEpoch            uint64 `json:"slotsPerEpoch"`
	warmup                   bool   `json:"warmup"`
}

// From Solana source logic
func (e *EpochSchedule) GetSlotsInEpoch(epoch Epoch) (slots uint64) {
	if epoch < e.FirstNormalEpoch {
		return uint64(math.Pow(2, float64(int(epoch)+bits.TrailingZeros(uint(MINIMUM_SLOTS_PER_EPOCH)))))
	} else {
		return e.SlotsPerEpoch
	}
}

func (e *EpochSchedule) GetFirstSlotInEpoch(epoch Epoch) (slot Slot) {
	if epoch <= e.FirstNormalEpoch {
		return Slot(uint64(math.Pow(2, float64(epoch))-1) * MINIMUM_SLOTS_PER_EPOCH)
	} else {
		return Slot(uint64(epoch-e.FirstNormalEpoch)*e.SlotsPerEpoch) + e.FirstNormalSlot
	}
}

func (e *EpochSchedule) GetLastSlotInEpoch(epoch Epoch) (slot Slot) {
	return e.GetFirstSlotInEpoch(epoch) + Slot(e.GetSlotsInEpoch(epoch)-1)
}

func (e *EpochSchedule) GetEpochAndSlotIndex(slot Slot) (epoch Epoch, slotIndex uint64) {
	if slot < e.FirstNormalSlot {
		/*
		   let epoch = (slot + MINIMUM_SLOTS_PER_EPOCH + 1)
		       .next_power_of_two()
		       .trailing_zeros()
		       - MINIMUM_SLOTS_PER_EPOCH.trailing_zeros()
		       - 1;

		   let epoch_len = 2u64.pow(epoch + MINIMUM_SLOTS_PER_EPOCH.trailing_zeros());

		   (
		       u64::from(epoch),
		       slot - (epoch_len - MINIMUM_SLOTS_PER_EPOCH),
		   )
		*/
		return 0, 0
	} else {
		return e.FirstNormalEpoch + Epoch(uint64(slot-e.FirstNormalSlot)/e.SlotsPerEpoch), uint64(slot-e.FirstNormalSlot) % e.SlotsPerEpoch
	}
}
