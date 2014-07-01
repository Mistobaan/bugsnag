package bugsnag

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
)

func init() {
	// Configure bugsnag
	DefaultClient.OSVersion = "3.2.1"
	DefaultClient.APIKey = os.Getenv("BUGSNAP_APIKEY")
}

func TestNotify(t *testing.T) {
	// Notify about an error
	e := errors.New("This is a test")
	if err := NotifyError(e); err != nil {
		t.Fatal(err)
	}
}

func TestNotifyRequest(t *testing.T) {
	// Notify about an error
	e := errors.New("This is a test")
	if r, err := http.NewRequest("GET", "some URL", nil); err != nil {
		t.Fatal(err)
	} else if err := NotifyErrorRequest(e, r); err != nil {
		t.Fatal(err)
	}
}

func TestSetMetaDataBeforeNotify(t *testing.T) {
	// Notify about another error, with more details
	e := errors.New("This is another test")
	values := map[string]interface{}{
		"account_id": 5555,
		"user_agent": "ie4",
	}
	if err := Notify(New(e).WithUserID("12345").WithMetaDataValues("user_info", values)); err != nil {
		t.Fatal(err)
	}
}

func TestWithMetaDataBeforeNotify(t *testing.T) {
	// Notify about another error, with more details
	e := errors.New("Another error")
	if err := Notify(New(e).WithUserID("12345").WithMetaData("user_info", "name", "mr. Fu")); err != nil {
		t.Fatal(err)
	}
}

func TestNewNotify(t *testing.T) {
	// Notify about another error, with more details
	e := errors.New("One more error")
	if err := Notify(New(e)); err != nil {
		t.Fatal(err)
	}
}

func TestCapturePanic(t *testing.T) {
	// Notify about an error
	r, err := http.NewRequest("GET", "some URL", nil)
	if err != nil {
		t.Fatal(err)
	}

	defer func() {
		if rec := recover(); rec != nil {
			if rec.(string) != "This should be reported!" {
				t.Fatal("Unexpected capture: " + rec.(string))
			}
		} else {
			t.Fatal("Panic not captured")
		}
	}()

	defer CapturePanic(r)

	panic("This should be reported!")
}

func TestStacktraceFunc(t *testing.T) {
	TraceFilterFunc = func(traces []Stacktrace) []Stacktrace {
		for i := 0; i < len(traces); i++ {
			traces[i].File = "this is not a string" + traces[i].File
		}
		return traces
	}
	defer func() {
		TraceFilterFunc = nil
	}()

	traces := stacktrace(3)
	for _, trace := range traces {
		if !strings.HasPrefix(trace.File, "this is not a string") {
			t.Fatal("TraceFilterFunc is not correctly filtering the stacktrace")
		}
	}

}
