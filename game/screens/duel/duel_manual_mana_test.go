package duel

import (
	"image"
	"testing"
	"time"
	"uuid"

	_ "github.com/benprew/mage-go/cards"
	mage "github.com/benprew/mage-go/pkg/mage"
	"github.com/benprew/mage-go/pkg/mage/interactive"
	"github.com/benprew/mage-go/pkg/mage/interactive/ai"
	"github.com/benprew/mage-go/pkg/mage/interactive/ai/heuristic"
	"github.com/hajimehoshi/ebiten/v2"
)

func TestDuelScreen_ManualManaTap_Pipeline(t *testing.T) {
	human := interactive.NewHumanPlayer("You")
	bot := ai.NewAIPlayer("Opponent", heuristic.New(ai.MidrangeWeighted))

	// Setup library with Mountain and Sol Ring for human
	mountain, err := mage.CreateCard("Mountain")
	if err != nil {
		t.Fatalf("CreateCard Mountain: %v", err)
	}
	human.AddToLibrary(mountain)

	solRing, err := mage.CreateCard("Sol Ring")
	if err != nil {
		t.Fatalf("CreateCard Sol Ring: %v", err)
	}
	human.AddToLibrary(solRing)

	for range 5 {
		c, _ := mage.CreateCard("Forest")
		bot.AddToLibrary(c)
	}

	g := newTestAnteGame(t, human, bot)

	// Draw Mountain and Sol Ring into hand
	human.DrawCard()
	human.DrawCard()
	for range 4 {
		bot.DrawCard()
	}

	go interactive.RunGameLoop(g, 0, 0)

	// 1. Wait for human priority to play Mountain
	var playMountain *interactive.ActionOption
	deadline := time.After(5 * time.Second)
	for playMountain == nil {
		select {
		case msg := <-human.ToTUI():
			for _, opt := range msg.Options {
				if opt.Type == interactive.ActionPlayLand && opt.CardName == "Mountain" {
					playMountain = &opt
					break
				}
			}
			if playMountain == nil && len(msg.Options) > 0 {
				human.FromTUI() <- interactive.PriorityAction{Type: interactive.ActionPass}
			}
		case <-deadline:
			t.Fatal("timed out waiting for PlayLand option")
		}
	}

	// Play Mountain
	human.FromTUI() <- interactive.PriorityAction{
		Type:   interactive.ActionPlayLand,
		CardID: playMountain.CardID,
	}

	// 2. Next message must contain ActionActivateAbility for Mountain
	var tapMountainOpt *interactive.ActionOption
	var lastMsg *interactive.GameMsg
	deadline = time.After(5 * time.Second)
	for tapMountainOpt == nil {
		select {
		case msg := <-human.ToTUI():
			lastMsg = &msg
			for _, opt := range msg.Options {
				if opt.Type == interactive.ActionActivateAbility && opt.CardName == "Mountain" {
					tapMountainOpt = &opt
					break
				}
			}
			if tapMountainOpt == nil && len(msg.Options) > 0 {
				human.FromTUI() <- interactive.PriorityAction{Type: interactive.ActionPass}
			}
		case <-deadline:
			t.Fatal("timed out waiting for ActionActivateAbility on Mountain")
		}
	}

	t.Logf("Received manual tap option: label=%q isManaAbility=%v permID=%s",
		tapMountainOpt.Label, tapMountainOpt.IsManaAbility, tapMountainOpt.PermanentID)

	if !tapMountainOpt.IsManaAbility {
		t.Errorf("expected IsManaAbility to be true")
	}

	// Test DuelScreen integration: cardActions populated and performCardAction dispatches
	duelScr := &DuelScreen{
		human:       human,
		cardActions: make(map[uuid.UUID][]interactive.ActionOption),
		self:        &duelPlayer{name: "You"},
		opponent:    &duelPlayer{name: "Opponent"},
		lastMsg:     lastMsg,
	}
	duelScr.refreshCardActions()

	mountainActions, ok := duelScr.cardActions[tapMountainOpt.PermanentID]
	if !ok || len(mountainActions) == 0 {
		t.Fatalf("duelScr.cardActions must contain actions for Mountain permanent")
	}
	if mountainActions[0].Type != interactive.ActionActivateAbility {
		t.Errorf("expected ActionActivateAbility, got %v", mountainActions[0].Type)
	}

	// Test UI border rendering: drawPermanentBorders executes without panic
	screen := ebiten.NewImage(1024, 768)
	permState := lastMsg.State.You.Battlefield[0]
	duelScr.drawPermanentBorders(screen, duelScr.self, permState, image.Pt(100, 100))

	// 3. Perform manual tap action by clicking Mountain (via performCardAction)
	duelScr.performCardAction(tapMountainOpt.PermanentID, "Mountain")

	// 4. Read next state from engine: Mountain is tapped, pool has {R: 1}, Sol Ring is castable!
	var castSolRingOpt *interactive.ActionOption
	deadline = time.After(5 * time.Second)
	for castSolRingOpt == nil {
		select {
		case msg := <-human.ToTUI():
			t.Logf("State after manual tap: Step=%s ManaPool(R)=%d Battlefield=%d",
				msg.State.Step, msg.State.You.ManaPool.Red, len(msg.State.You.Battlefield))

			if msg.State.You.ManaPool.Red != 1 {
				t.Fatalf("expected 1 Red mana in pool, got %d", msg.State.You.ManaPool.Red)
			}
			if len(msg.State.You.Battlefield) > 0 && !msg.State.You.Battlefield[0].Tapped {
				t.Errorf("expected Mountain on battlefield to be tapped")
			}

			// Mountain should no longer be activatable for mana while tapped
			for _, opt := range msg.Options {
				if opt.Type == interactive.ActionActivateAbility && opt.PermanentID == tapMountainOpt.PermanentID {
					t.Errorf("tapped Mountain must not have an active tap action")
				}
				if opt.Type == interactive.ActionCastSpell && opt.CardName == "Sol Ring" {
					castSolRingOpt = &opt
				}
			}

			if castSolRingOpt == nil && len(msg.Options) > 0 {
				human.FromTUI() <- interactive.PriorityAction{Type: interactive.ActionPass}
			}
		case <-deadline:
			t.Fatal("timed out waiting for state after manual tap")
		}
	}

	t.Logf("SUCCESS: Found CastSpell for Sol Ring (%s) using mana from manual tap!", castSolRingOpt.CardID)

	// 5. Cast Sol Ring: solver spends the 1 {R} from pool, and Sol Ring resolves to battlefield
	human.FromTUI() <- interactive.PriorityAction{
		Type:   interactive.ActionCastSpell,
		CardID: castSolRingOpt.CardID,
	}

	// Wait for spell cast and priority
	deadline = time.After(5 * time.Second)
	solRingOnField := false
	for !solRingOnField {
		select {
		case msg := <-human.ToTUI():
			t.Logf("State after cast: Step=%s ManaPool(R)=%d Battlefield=%d",
				msg.State.Step, msg.State.You.ManaPool.Red, len(msg.State.You.Battlefield))
			for _, perm := range msg.State.You.Battlefield {
				if perm.Name == "Sol Ring" {
					solRingOnField = true
					break
				}
			}
			if !solRingOnField && len(msg.Options) > 0 {
				human.FromTUI() <- interactive.PriorityAction{Type: interactive.ActionPass}
			}
		case <-deadline:
			t.Fatal("timed out waiting for Sol Ring to enter battlefield")
		}
	}

	t.Logf("SUCCESS: Sol Ring successfully entered battlefield paid by manual mana pool tap!")
}
