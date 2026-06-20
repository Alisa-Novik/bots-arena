package core

import (
	"golab/internal/util"
	"testing"
)

func TestHealMemberRequiresControllerAmountOrFood(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	bot.ConnnectedToColony = true
	bot.Hp = 50
	ctrl := Controller{Amount: 0}

	colony.HealMember(&bot, &ctrl)

	if bot.Hp != 50 {
		t.Fatalf("hp after depleted heal = %d, want 50", bot.Hp)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount = %d, want 0", ctrl.Amount)
	}

	ctrl.Amount = 2
	colony.HealMember(&bot, &ctrl)

	if bot.Hp != 53 {
		t.Fatalf("hp after funded heal = %d, want 53", bot.Hp)
	}
	if ctrl.Amount != 1 {
		t.Fatalf("controller amount after funded heal = %d, want 1", ctrl.Amount)
	}
}

func TestHealMemberCanSpendFoodWhenControllerIsEmpty(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	bot.ConnnectedToColony = true
	bot.Hp = 50
	bot.Inventory.Food = 2
	bot.Inventory.Ore = 2
	ctrl := Controller{Amount: 0}

	colony.HealMember(&bot, &ctrl)

	if bot.Hp != 65 {
		t.Fatalf("hp after food-funded heal = %d, want 65", bot.Hp)
	}
	if bot.Inventory.Food != 1 {
		t.Fatalf("food after heal = %d, want 1", bot.Inventory.Food)
	}
	if bot.Inventory.Ore != 2 {
		t.Fatalf("ore after heal = %d, want unchanged 2", bot.Inventory.Ore)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount after heal = %d, want 0", ctrl.Amount)
	}
}

func TestHealMemberDoesNotSpendOreWhenControllerIsEmpty(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	bot.ConnnectedToColony = true
	bot.Hp = 50
	bot.Inventory.Ore = 2
	ctrl := Controller{Amount: 0}

	colony.HealMember(&bot, &ctrl)

	if bot.Hp != 50 {
		t.Fatalf("hp after ore-only heal = %d, want unchanged 50", bot.Hp)
	}
	if bot.Inventory.Ore != 2 {
		t.Fatalf("ore after heal = %d, want unchanged 2", bot.Inventory.Ore)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount after heal = %d, want 0", ctrl.Amount)
	}
}

func TestHealMemberCanSpendBankFoodWhenControllerIsEmpty(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	bot.Hp = 50
	colony.FoodBank = 1
	ctrl := Controller{Amount: 0}

	colony.HealMember(&bot, &ctrl)

	if bot.Hp != 53 {
		t.Fatalf("hp after bank-funded heal = %d, want 53", bot.Hp)
	}
	if bot.Inventory.Food != 0 {
		t.Fatalf("personal food after heal = %d, want 0", bot.Inventory.Food)
	}
	if colony.FoodBank != 0 {
		t.Fatalf("food bank after heal = %d, want 0", colony.FoodBank)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount after heal = %d, want 0", ctrl.Amount)
	}
}

func TestDepositMemberSurplusKeepsDivisionReserve(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	colony.AddFamily(&bot)
	bot.ConnnectedToColony = true
	bot.Inventory = Inventory{Food: 5, Ore: 4}

	colony.DepositMemberSurplus(&bot, 1, 2)

	if bot.Inventory.Food != 1 || bot.Inventory.Ore != 2 {
		t.Fatalf("member inventory after deposit = %+v, want food 1 ore 2", bot.Inventory)
	}
	if colony.FoodBank != 4 || colony.OreBank != 2 {
		t.Fatalf("colony bank after deposit = F%d O%d, want F4 O2", colony.FoodBank, colony.OreBank)
	}
}

func TestDisconnectedOrColonylessBotsCannotUseBank(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	disconnected := NewBot(util.NewPos(10, 11))
	colony.AddFamily(&disconnected)
	disconnected.Inventory = Inventory{Food: 4, Ore: 4}
	colony.FoodBank = 10
	colony.OreBank = 10

	colony.DepositMemberSurplus(&disconnected, 1, 1)

	if disconnected.Inventory.Food != 4 || disconnected.Inventory.Ore != 4 {
		t.Fatalf("disconnected inventory after deposit = %+v, want unchanged 4/4", disconnected.Inventory)
	}
	if colony.FoodBank != 10 || colony.OreBank != 10 {
		t.Fatalf("bank after disconnected deposit = F%d O%d, want F10 O10", colony.FoodBank, colony.OreBank)
	}
	if colony.SpendWithBank(&disconnected, 5, 5) {
		t.Fatalf("disconnected bot spent bank resources")
	}
	if colony.FoodBank != 10 || colony.OreBank != 10 {
		t.Fatalf("bank after disconnected spend = F%d O%d, want F10 O10", colony.FoodBank, colony.OreBank)
	}

	colonyless := NewBot(util.NewPos(10, 12))
	if CanPayWithBank(&colonyless, 1, 0) {
		t.Fatalf("colonyless bot should not see colony bank")
	}
	if SpendWithBank(&colonyless, 1, 0) {
		t.Fatalf("colonyless bot spent missing personal food")
	}
}

func TestAddFamilyRecursivelyAssignsDescendantsWithoutDuplicates(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	parent := NewBot(util.NewPos(10, 11))
	child := NewBot(util.NewPos(10, 12))
	grandchild := NewBot(util.NewPos(10, 13))
	parent.AddOffspring(&child)
	child.AddOffspring(&grandchild)

	colony.AddFamily(&parent)
	colony.AddFamily(&parent)

	for _, bot := range []*Bot{&parent, &child, &grandchild} {
		if bot.Colony != &colony {
			t.Fatalf("bot %p colony = %p, want %p", bot, bot.Colony, &colony)
		}
	}
	if len(colony.Members) != 3 {
		t.Fatalf("colony member count = %d, want 3 unique family members", len(colony.Members))
	}
}

func TestAddFamilyRemovesTransferredBotFromOldColony(t *testing.T) {
	oldColony := NewColony(util.NewPos(10, 10))
	newColony := NewColony(util.NewPos(12, 12))
	bot := NewBot(util.NewPos(10, 11))

	oldColony.AddFamily(&bot)
	newColony.AddFamily(&bot)

	if bot.Colony != &newColony {
		t.Fatalf("bot colony = %p, want new colony %p", bot.Colony, &newColony)
	}
	if len(oldColony.Members) != 0 {
		t.Fatalf("old colony members = %d, want 0 after transfer", len(oldColony.Members))
	}
	if len(newColony.Members) != 1 || newColony.Members[0] != &bot {
		t.Fatalf("new colony members = %+v, want transferred bot", newColony.Members)
	}
}

func TestSetPathToWaterClearsEmptyPathWithoutIndexes(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	path := []util.Position{util.NewPos(10, 11), util.NewPos(10, 12)}

	colony.SetPathToWater(path)
	if !colony.IsPathToWater(path[0]) {
		t.Fatalf("path cell was not marked")
	}
	if got, ok := colony.PathToWaterIndex(path[1]); !ok || got != 1 {
		t.Fatalf("path index = %d/%v, want 1/true", got, ok)
	}

	colony.SetPathToWater(nil)
	if len(colony.PathToWater) != 0 || len(colony.pathToWaterMask) != 0 || len(colony.pathToWaterIndex) != 0 {
		t.Fatalf("empty path left cached arrays: path=%d mask=%d index=%d", len(colony.PathToWater), len(colony.pathToWaterMask), len(colony.pathToWaterIndex))
	}
	if colony.IsPathToWater(path[0]) {
		t.Fatalf("cleared path still marks path cell")
	}
}

func TestFlagHealingRequiresControllerAmount(t *testing.T) {
	colony := NewColony(util.NewPos(10, 10))
	bot := NewBot(util.NewPos(10, 11))
	bot.Hp = 50
	colony.AddMember(&bot)
	colony.AddFlag(&ColonyFlag{Pos: util.NewPos(10, 10)})
	ctrl := Controller{Amount: 0}

	colony.HealBotsInFlagRadius(5, 1, &ctrl)

	if bot.Hp != 50 {
		t.Fatalf("hp after depleted flag heal = %d, want 50", bot.Hp)
	}

	ctrl.Amount = 1
	colony.HealBotsInFlagRadius(5, 1, &ctrl)

	if bot.Hp != 51 {
		t.Fatalf("hp after funded flag heal = %d, want 51", bot.Hp)
	}
	if ctrl.Amount != 0 {
		t.Fatalf("controller amount after flag heal = %d, want 0", ctrl.Amount)
	}
}
