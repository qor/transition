package transition

import (
	"errors"
	"fmt"
	"os"
	"testing"

	_ "github.com/mattn/go-sqlite3"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Order struct {
	ID      int
	Address string
	Transition
}

var db = testDB()

func testDB() *gorm.DB {
	var db *gorm.DB
	var err error
	var dbuser, dbpwd, dbname, dbhost = "qor", "qor", "qor_test", "localhost"

	if os.Getenv("DB_USER") != "" {
		dbuser = os.Getenv("DB_USER")
	}

	if os.Getenv("DB_PWD") != "" {
		dbpwd = os.Getenv("DB_PWD")
	}

	if os.Getenv("DB_NAME") != "" {
		dbname = os.Getenv("DB_NAME")
	}

	if os.Getenv("DB_HOST") != "" {
		dbhost = os.Getenv("DB_HOST")
	}

	if os.Getenv("TEST_DB") == "postgres" {
		db, err = gorm.Open(postgres.New(postgres.Config{
			DSN: fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable", dbuser, dbpwd, dbhost, dbname),
		}), &gorm.Config{})
	} else if os.Getenv("TEST_DB") == "mysql" {
		// CREATE USER 'qor'@'localhost' IDENTIFIED BY 'qor';
		// CREATE DATABASE qor_test;
		// GRANT ALL ON qor_test.* TO 'qor'@'localhost';
		db, err = gorm.Open(mysql.New(mysql.Config{
			DSN: fmt.Sprintf("%s:%s@/%s?charset=utf8&parseTime=True&loc=Local", dbuser, dbpwd, dbname),
		}), &gorm.Config{})
	} else {
		os.Remove("test.db")
		db, err = gorm.Open(sqlite.Open("test.db"), &gorm.Config{})
	}

	if err != nil {
		panic(err)
	}

	return db
}

func init() {
	for _, model := range []interface{}{&Order{}, &StateChangeLog{}} {
		var migrator = db.Migrator()
		if err := migrator.CreateTable(model); err != nil {
			panic(err)
		}
	}
}

func getStateMachine() *StateMachine {
	var orderStateMachine = New(&Order{})

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

func CreateOrderAndExecuteTransition(transition *StateMachine, event string, order *Order) error {
	if err := db.Save(order).Error; err != nil {
		return err
	}

	if err := transition.Trigger(event, order, db); err != nil {
		return err
	}
	return nil
}

func TestStateTransition(t *testing.T) {
	order := &Order{}

	if err := getStateMachine().Trigger("checkout", order, db); err != nil {
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if order.GetState() != "checkout" {
		t.Errorf("state doesn't changed to checkout")
	}

	var stateChangeLogs = GetStateChangeLogs(order, db)
	if len(stateChangeLogs) != 1 {
		t.Errorf("should get one state change log with GetStateChangeLogs")
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

	var lastStateChange = GetLastStateChange(order, db)
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
	if err := orderStateMachine.Trigger("cancel", unpaidOrder1, db); err != nil {
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if unpaidOrder1.State != "cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}

	unpaidOrder2 := &Order{}
	unpaidOrder2.State = "draft"
	if err := orderStateMachine.Trigger("cancel", unpaidOrder2, db); err != nil {
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if unpaidOrder2.State != "cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}

	paidOrder := &Order{}
	paidOrder.State = "paid"
	if err := orderStateMachine.Trigger("cancel", paidOrder, db); err != nil {
		t.Errorf("should not raise any error when trigger event cancel")
	}

	if paidOrder.State != "paid_cancelled" {
		t.Errorf("order status doesn't transitioned correctly")
	}
}

func TestStateCallbacks(t *testing.T) {
	orderStateMachine := getStateMachine()
	order := &Order{}

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
		t.Errorf("should not raise any error when trigger event checkout")
	}

	if order.Address != address1 {
		t.Errorf("enter callback not triggered")
	}

	if err := orderStateMachine.Trigger("pay", order, db); err != nil {
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
