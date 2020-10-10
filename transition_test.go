package transition_test

import (
	"errors"
	"testing"

	"github.com/jinzhu/gorm"
	"github.com/qor/qor/test/utils"
	"github.com/qor/transition"
)

type Order struct {
	Id      int
	Address string

	transition.Transition
}

var db = utils.TestDB()

func init() {
	for _, model := range []interface{}{&Order{}, &transition.StateChangeLog{}} {
		if err := db.DropTableIfExists(model).Error; err != nil {
			panic(err)
		}

		if err := db.AutoMigrate(model).Error; err != nil {
			panic(err)
		}
	}
}

func getStateMachine() *transition.StateMachine {
	var orderStateMachine = transition.New(&Order{})

	orderStateMachine.Initial("draft")
	orderStateMachine.State("checkout")
	orderStateMachine.State("paid")
	orderStateMachine.State("processed")
	orderStateMachine.State("delivered")
	orderStateMachine.State("cancelled")
	orderStateMachine.State("paid_cancelled")

	orderStateMachine.Event("checkout").To("checkout").From("draft")
	orderStateMachine.Event("pay").To("paid").From("checkout")

	return orderStateMachine
}

func TestStateTransition(t *testing.T) {
	order := &Order{}
	order.Address = t.Name()

	if err := getStateMachine().Trigger("checkout", order, db); err != nil {
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if order.GetState() != "checkout" {
		t.Errorf("state doesn't changed to checkout")
	}

	stateChangeLogs, err := transition.GetStateChangeLogs(order, db)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(stateChangeLogs); n != 1 {
		t.Errorf("should get one state change log with GetStateChangeLogs got %d", n)
	} else {
		var stateChangeLog = stateChangeLogs[0]

		if stateChangeLog.From != "draft" {
			t.Errorf("state from not set")
		}

		if stateChangeLog.To != "checkout" {
			t.Errorf("state to not set")
		}
	}
}

func TestGetLastStateChange(t *testing.T) {
	order := &Order{}

	if err := getStateMachine().Trigger("checkout", order, db, "checkout note"); err != nil {
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if err := getStateMachine().Trigger("pay", order, db, "pay note"); err != nil {
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if order.GetState() != "paid" {
		t.Errorf("state doesn't changed to paid")
	}

	var lastStateChange = transition.GetLastStateChange(order, db)
	if lastStateChange.To != "paid" {
		t.Errorf("state to not set")
	} else {
		if lastStateChange.From != "checkout" {
			t.Errorf("state from not set")
		}

		if lastStateChange.Note != "pay note" {
			t.Errorf("state note not set")
		}
	}
}

func TestMultipleTransitionWithOneEvent(t *testing.T) {
	orderStateMachine := getStateMachine()
	cancellEvent := orderStateMachine.Event("cancel")
	cancellEvent.To("cancelled").From("draft", "checkout")
	cancellEvent.To("paid_cancelled").From("paid", "processed")

	unpaidOrder1 := &Order{}
	unpaidOrder1.Address = t.Name() + ":unpaid1"
	if err := orderStateMachine.Trigger("cancel", unpaidOrder1, db); err != nil {
		t.Error(err)
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if unpaidOrder1.State != "cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}

	unpaidOrder2 := &Order{}
	unpaidOrder2.Address = t.Name() + ":unpaid2"
	unpaidOrder2.State = "draft"
	if err := orderStateMachine.Trigger("cancel", unpaidOrder2, db); err != nil {
		t.Error(err)
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if unpaidOrder2.State != "cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}

	paidOrder := &Order{}
	paidOrder.Address = t.Name() + ":paid"
	paidOrder.State = "paid"
	if err := orderStateMachine.Trigger("cancel", paidOrder, db); err != nil {
		t.Error(err)
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if paidOrder.State != "paid_cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}
}

func TestStateCallbacks(t *testing.T) {
	orderStateMachine := getStateMachine()
	order := &Order{Address: t.Name()}

	address1 := "I'm an address should be set when enter checkout"
	address2 := "I'm an address should be set when exit checkout"
	orderStateMachine.State("checkout").Enter(func(order interface{}, tx *gorm.DB) error {
		order.(*Order).Address = address1
		return nil
	}).Exit(func(order interface{}, tx *gorm.DB) error {
		order.(*Order).Address = address2
		return nil
	})

	if err := orderStateMachine.Trigger("checkout", order, db); err != nil {
		t.Error(err)
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if order.Address != address1 {
		t.Errorf("enter callback not triggered")
	}

	if err := orderStateMachine.Trigger("pay", order, db); err != nil {
		t.Error(err)
		t.Errorf("should not raise any error when trigger event pay")
	}

	if order.Address != address2 {
		t.Errorf("exit callback not triggered")
	}
}

func TestEventCallbacks(t *testing.T) {
	var (
		order                 = &Order{}
		orderStateMachine     = getStateMachine()
		prevState, afterState string
	)

	orderStateMachine.Event("checkout").To("checkout").From("draft").Before(func(order interface{}, tx *gorm.DB) error {
		prevState = order.(*Order).State
		return nil
	}).After(func(order interface{}, tx *gorm.DB) error {
		afterState = order.(*Order).State
		return nil
	})

	order.State = "draft"
	if err := orderStateMachine.Trigger("checkout", order, nil); err != nil {
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if prevState != "draft" {
		t.Errorf("Before callback triggered after state change")
	}

	if afterState != "checkout" {
		t.Errorf("After callback triggered after state change")
	}
}

func TestTransitionOnEnterCallbackError(t *testing.T) {
	var (
		order             = &Order{}
		orderStateMachine = getStateMachine()
	)

	orderStateMachine.State("checkout").Enter(func(order interface{}, tx *gorm.DB) (err error) {
		return errors.New("intentional error")
	})

	if err := orderStateMachine.Trigger("checkout", order, nil); err == nil {
		t.Errorf("should raise an intentional error")
	}

	if order.State != "draft" {
		t.Errorf("state transitioned on Enter callback error")
	}
}

func TestTransitionOnExitCallbackError(t *testing.T) {
	var (
		order             = &Order{}
		orderStateMachine = getStateMachine()
	)

	orderStateMachine.State("checkout").Exit(func(order interface{}, tx *gorm.DB) (err error) {
		return errors.New("intentional error")
	})

	if err := orderStateMachine.Trigger("checkout", order, nil); err != nil {
		t.Errorf("should not raise error when checkout")
	}

	if err := orderStateMachine.Trigger("pay", order, nil); err == nil {
		t.Errorf("should raise an intentional error")
	}

	if order.State != "checkout" {
		t.Errorf("state transitioned on Enter callback error")
	}
}

func TestEventOnBeforeCallbackError(t *testing.T) {
	var (
		order             = &Order{}
		orderStateMachine = getStateMachine()
	)

	orderStateMachine.Event("checkout").To("checkout").From("draft").Before(func(order interface{}, tx *gorm.DB) error {
		return errors.New("intentional error")
	})

	if err := orderStateMachine.Trigger("checkout", order, nil); err == nil {
		t.Errorf("should raise an intentional error")
	}

	if order.State != "draft" {
		t.Errorf("state transitioned on Enter callback error")
	}
}

func TestEventOnAfterCallbackError(t *testing.T) {
	var (
		order             = &Order{}
		orderStateMachine = getStateMachine()
	)

	orderStateMachine.Event("checkout").To("checkout").From("draft").After(func(order interface{}, tx *gorm.DB) error {
		return errors.New("intentional error")
	})

	if err := orderStateMachine.Trigger("checkout", order, nil); err == nil {
		t.Errorf("should raise an intentional error")
	}

	if order.State != "draft" {
		t.Errorf("state transitioned on Enter callback error")
	}
}
