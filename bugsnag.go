package bugsnag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"
	"runtime"
	"strconv"
)

var (
	APIKey              string
	AppVersion          string
	OSVersion           string
	ReleaseStage        = "production"
	NotifyReleaseStages = []string{"production"}
	AutoNotify          = true
	UseSSL              = true
	Verbose             = false
	Hostname            string
	DefaultNotifier            = &Notifier{
		Name:    "Bugsnag Go client",
		Version: "0.1",
		URL:     "https://github.com/Mistobaan/bugsnag",
	}
	TraceFilterFunc StacktraceFunc
)

type Notifier struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

type Payload struct {
	APIKey   string           `json:"apiKey"`
	Notifier *Notifier `json:"notifier"`
	Events   []*Event         `json:"events"`
}

type Exception struct {
	ErrorClass string       `json:"errorClass"`
	Message    string       `json:"message,omitempty"`
	Stacktrace []Stacktrace `json:"stacktrace,omitempty"`
}

type Stacktrace struct {
	File       string `json:"file"`
	LineNumber string `json:"lineNumber"`
	Method     string `json:"method"`
	InProject  bool   `json:"inProject,omitempty"`
}

type Event struct {
	UserID       string                            `json:"userId,omitempty"`
	AppVersion   string                            `json:"appVersion,omitempty"`
	OSVersion    string                            `json:"osVersion,omitempty"`
	ReleaseStage string                            `json:"releaseStage"`
	Context      string                            `json:"context,omitempty"`
	Exceptions   []Exception                       `json:"exceptions"`
	MetaData     map[string]map[string]interface{} `json:"metaData,omitempty"`
}

type StacktraceFunc func(traces []Stacktrace) []Stacktrace

const ApplicationJson = "application/json"

func send(events []*Event) error {
	payload := &Payload{
		Notifier: DefaultNotifier,
		APIKey:   APIKey,
		Events:   events,
	}
	protocol := "http"
	if UseSSL {
		protocol = "https"
	}
	b, err := json.MarshalIndent(payload, "", "\t")
	if err != nil {
		return err
	}
	resp, err := http.Post(protocol+"://notify.bugsnag.com", ApplicationJson, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	// Always close a response's Body (which is always non-nil if err==nil)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	} else if Verbose {
		println(string(b))
		b, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		println(resp.StatusCode)
		println(resp.Status)
		println(string(b))
	}
	return nil
}

func getStacktrace() []Stacktrace {
	var stacktrace []Stacktrace
	i := 3 // First 3 lines are our own functions, not interesting
	for {
		if pc, file, line, ok := runtime.Caller(i); !ok {
			break
		} else {
			methodName := "unnamed"
			if f := runtime.FuncForPC(pc); f != nil {
				methodName = f.Name()
			}
			if methodName != "panic" {
				traceLine := Stacktrace{
					File:       file,
					LineNumber: strconv.Itoa(line),
					Method:     methodName,
				}
				stacktrace = append(stacktrace, traceLine)
			}
		}
		i += 1
	}
	if TraceFilterFunc != nil {
		stacktrace = TraceFilterFunc(stacktrace)
	}
	return stacktrace
}

// Notify sends an error to bugsnag
func Notify(err error) error {
	return New(err).Notify()
}

// NotifyRequest sends an error to bugsnag, and sets request
// URL as the event context
// and marshals down the request content.
func NotifyRequest(err error, r *http.Request) error {
	return New(err).WithContext(r.URL.String()).WithMetaData("request", "dump", r).Notify()
}

// CapturePanic reports panics happening while processing a HTTP request
func CapturePanic(r *http.Request) {
	if recovered := recover(); recovered != nil {
		if err, ok := recovered.(error); ok {
			NotifyRequest(err, r)
		} else if err, ok := recovered.(string); ok {
			NotifyRequest(errors.New(err), r)
		}
		panic(recovered)
	}
}

// New returns returns a bugsnag event instance, that can be further configured
// before sending it.
func New(err error) *Event {
	return &Event{
		AppVersion:   AppVersion,
		OSVersion:    OSVersion,
		ReleaseStage: ReleaseStage,
		Exceptions: []Exception{
			Exception{
				ErrorClass: reflect.TypeOf(err).String(),
				Message:    err.Error(),
				Stacktrace: getStacktrace(),
			},
		},
	}
}

// WithUserID sets the user_id property on the bugsnag event.
func (event *Event) WithUserID(userID string) *Event {
	event.UserID = userID
	return event
}

func (event *Event) WithContext(context string) *Event {
	event.Context = context
	return event
}

// WithMetaDataValues sets bunch of key-value pairs under a tab in bugsnag
func (event *Event) WithMetaDataValues(tab string, values map[string]interface{}) *Event {
	if event.MetaData == nil {
		event.MetaData = make(map[string]map[string]interface{})
	}
	event.MetaData[tab] = values
	return event
}

// WithMetaData adds a key-value pair under a tab in bugsnag
func (event *Event) WithMetaData(tab string, name string, value interface{}) *Event {
	if event.MetaData == nil {
		event.MetaData = make(map[string]map[string]interface{})
	}
	if event.MetaData[tab] == nil {
		event.MetaData[tab] = make(map[string]interface{})
	}
	event.MetaData[tab][name] = value
	return event
}

// Notify sends the configured event off to bugsnag.
func (event *Event) Notify() error {
	for _, stage := range NotifyReleaseStages {
		if stage == event.ReleaseStage {
			if Hostname != "" {
				// Custom metadata to know what machine is reporting the error.
				event.WithMetaData("host", "name", Hostname)
			}
			return send([]*Event{event})
		}
	}
	return nil
}
