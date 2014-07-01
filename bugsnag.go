package bugsnag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"runtime"
	"strings"
)

const bugsnagURL string = "notify.bugsnag.com"
const applicationJson = "application/json"

func hostname() string {
	name, _ := os.Hostname()
	return name
}

func init() {
	DefaultClient.APIKey = os.Getenv("BUGSNAG_APIKEY")
}

var (
	// Default Notifier
	DefaultNotifier = &Notifier{
		Name:    "Bugsnag Go",
		Version: "0.1",
		URL:     "https://github.com/Mistobaan/bugsnag",
	}

	// Filter function
	TraceFilterFunc StacktraceFunc

	// Default Client to use
	DefaultClient *Client = &Client{
		ReleaseStage:        "production",
		NotifyReleaseStages: []string{"production"},
		AutoNotify:          true,
		UseSSL:              true,
		Verbose:             false,
		Indent:              false,
		Url:                 bugsnagURL,
		Hostname:            hostname(),
		Notifier:            DefaultNotifier,
	}
)

// Client bugsnag notification api
type Client struct {
	APIKey              string
	OSVersion           string
	Protocol            string
	AutoNotify          bool
	UseSSL              bool
	Verbose             bool
	Url                 string
	Indent              bool
	ReleaseStage        string
	NotifyReleaseStages []string
	Hostname            string
	Notifier            *Notifier
	DefaultContext      string
	App                 *App
}

type App struct {
	Version      string `json:"version"`
	ReleaseStage string `json:"releaseStage"`
}

type Device struct {
	OSVersion string `json:"osVersion"`
	hostname  string `json:"hostname"`
}

type Notifier struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	URL     string `json:"url"`
}

type Payload struct {
	APIKey   string    `json:"apiKey"`
	Notifier *Notifier `json:"notifier"`
	Events   []*Event  `json:"events"`
}

type Exception struct {
	ErrorClass string       `json:"errorClass"`
	Message    string       `json:"message,omitempty"`
	Stacktrace []Stacktrace `json:"stacktrace,omitempty"`
}

type Stacktrace struct {
	File       string `json:"file"`
	LineNumber int    `json:"lineNumber"`
	Method     string `json:"method"`
	InProject  bool   `json:"inProject,omitempty"`
}

type Event struct {
	UserID         string                            `json:"userId,omitempty"`
	PayloadVersion string                            `json:"payloadVersion"`
	App            *App                              `json:"app,omitempty"`
	Device         *Device                           `json:"device,omitempty"`
	OSVersion      string                            `json:"osVersion,omitempty"`
	ReleaseStage   string                            `json:"releaseStage"`
	Context        string                            `json:"context,omitempty"`
	Exceptions     []Exception                       `json:"exceptions"`
	MetaData       map[string]map[string]interface{} `json:"metaData,omitempty"`
}

type StacktraceFunc func(traces []Stacktrace) []Stacktrace

func encode(payload interface{}, indent bool) ([]byte, error) {

	if indent {
		b, err := json.MarshalIndent(payload, "", "\t")
		if err != nil {
			return nil, err
		}
		return b, nil
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return b, nil
}

func (c *Client) send(events []*Event) error {

	if c.APIKey == "" {
		return fmt.Errorf("No Api Key Provided")
	}

	payload := &Payload{
		Notifier: c.Notifier,
		APIKey:   c.APIKey,
		Events:   events,
	}

	protocol := "http://"
	if c.UseSSL {
		protocol = "https://"
	}

	b, err := encode(payload, c.Indent)
	if err != nil {
		return err
	}
	resp, err := http.Post(protocol+c.Url, applicationJson, bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	// Always close a response's Body (which is always non-nil if err==nil)
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("Unexpected status code: %d", resp.StatusCode)
	}
	return nil
}

// New returns returns a bugsnag Event instance, that can be further configured
// before sending it.
func (c *Client) New(err error) *Event {
	return &Event{
		PayloadVersion: "2",
		OSVersion:      c.OSVersion,
		ReleaseStage:   c.ReleaseStage,
		App:            c.App,
		// XXX Context
		// XXX GroupingHash
		// XXX Severity

		// XXX USER suport

		// AppVersion

		Exceptions: []Exception{
			Exception{
				ErrorClass: reflect.TypeOf(err).String(),
				Message:    err.Error(),
				Stacktrace: stacktrace(3),
			},
		},
	}
}

func SetAPIKey(key string) {
	DefaultClient.APIKey = key
}

func (c *Client) SetApp(app *App) {
	c.App = app
}

func SetApp(app *App) {
	DefaultClient.SetApp(app)
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
func (c *Client) Notify(event *Event) error {
	for _, stage := range c.NotifyReleaseStages {
		if stage == event.ReleaseStage {
			if c.Hostname != "" {
				// Custom metadata to know what machine is reporting the error.
				event.WithMetaData("host", "name", c.Hostname)
			}
			return c.send([]*Event{event})
		}
	}
	return nil
}

func New(e error) *Event {
	return DefaultClient.New(e)
}

// NotifyError sends an error to bugsnag using the default client
func NotifyError(err error) error {
	return DefaultClient.Notify(DefaultClient.New(err))
}

// Notify sends an event to bugsnag using the default client
func Notify(e *Event) error {
	return DefaultClient.Notify(e)
}

// NotifyRequestError sends an error to bugsnag, and sets request
// URL as the event context
// and marshals down the request content.
func NotifyRequestError(err error, r *http.Request) error {
	event := DefaultClient.New(err).WithContext(r.URL.String()).WithMetaData("request", "dump", r)
	return DefaultClient.Notify(event)
}

const (
	centerDot = "·"
	dot       = "."
	dunno     = "???"
)

// function returns, if possible, the name of the function containing the PC.
func function(pc uintptr) string {
	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return dunno
	}
	name := fn.Name()
	// Strip package path name
	if period := strings.Index(name, dot); period >= 0 {
		name = name[period+1:]
	}
	return strings.Replace(name, centerDot, dot, -1)
}

// TODO strip basedir
func stacktrace(skip int) []Stacktrace {
	var stacktrace []Stacktrace
	for i := skip; ; i++ {
		pc, file, line, ok := runtime.Caller(i)
		if !ok {
			break
		}

		methodName := function(pc)

		if methodName != "panic" {
			traceLine := Stacktrace{
				File:       file,
				LineNumber: line,
				Method:     methodName,
				InProject:  !strings.Contains(file, "go/src/pkg/"),
			}
			stacktrace = append(stacktrace, traceLine)
		}

	}
	if TraceFilterFunc != nil {
		stacktrace = TraceFilterFunc(stacktrace)
	}
	return stacktrace
}

// CapturePanic reports panics happening while processing an HTTP request
func CapturePanic(r *http.Request) {
	if recovered := recover(); recovered != nil {
		if err, ok := recovered.(error); ok {
			NotifyRequestError(err, r)
		} else if err, ok := recovered.(string); ok {
			NotifyRequestError(errors.New(err), r)
		}
		panic(recovered)
	}
}
