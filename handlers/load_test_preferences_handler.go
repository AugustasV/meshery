//Package handlers :  collection of handlers (aka "HTTP middleware")
package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/layer5io/meshery/models"
	SMPS "github.com/layer5io/service-mesh-performance-specification/spec"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// LoadTestPrefencesHandler is used for persisting load test preferences
func (h *Handler) LoadTestPrefencesHandler(w http.ResponseWriter, req *http.Request, prefObj *models.Preference, user *models.User, provider models.Provider) {
	if req.Method != http.MethodPost {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	q := req.FormValue("qps")
	qps, err := strconv.Atoi(q)
	if err != nil {
		err = errors.Wrap(err, "unable to parse qps")
		logrus.Error(err)
		http.Error(w, "please provide a valid value for qps", http.StatusBadRequest)
		return
	}
	if qps < 0 {
		http.Error(w, "please provide a valid value for qps", http.StatusBadRequest)
		return
	}
	dur := req.FormValue("t")
	if _, err = time.ParseDuration(dur); err != nil {
		err = errors.Wrap(err, "unable to parse t as a duration")
		logrus.Error(err)
		http.Error(w, "please provide a valid value for t", http.StatusBadRequest)
		return
	}
	cu := req.FormValue("c")
	c, err := strconv.Atoi(cu)
	if err != nil {
		err = errors.Wrap(err, "unable to parse c")
		logrus.Error(err)
		http.Error(w, "please provide a valid value for c", http.StatusBadRequest)
		return
	}
	if c < 0 {
		http.Error(w, "please provide a valid value for c", http.StatusBadRequest)
		return
	}
	gen := req.FormValue("gen")
	genTrack := false
	// TODO: after we have interfaces for load generators in place, we need to make a generic check, for now using a hard coded one
	for _, lg := range []models.LoadGenerator{models.FortioLG, models.Wrk2LG} {
		if lg.Name() == gen {
			genTrack = true
		}
	}
	if !genTrack {
		logrus.Error("invalid value for gen")
		http.Error(w, "please provide a valid value for gen (load generator)", http.StatusBadRequest)
		return
	}
	prefObj.LoadTestPreferences = &models.LoadTestPreferences{
		ConcurrentRequests: c,
		Duration:           dur,
		QueriesPerSecond:   qps,
		LoadGenerator:      gen,
	}
	if err = provider.RecordPreferences(req, user.UserID, prefObj); err != nil {
		logrus.Errorf("unable to save user preferences: %v", err)
		http.Error(w, "unable to save user preferences", http.StatusInternalServerError)
		return
	}

	_, _ = w.Write([]byte("{}"))
}

type custTestConf struct {
	Val *SMPS.PerformanceTestConfig
}

func (c *custTestConf) MarshalJSON() ([]byte, error) {
	m := jsonpb.Marshaler{
		EmitDefaults: true,
	}
	val, err := m.MarshalToString(c.Val)
	return []byte(val), err
}

// UserTestPreferenceHandler is used for persisting load test preferences
func (h *Handler) UserTestPreferenceStore(w http.ResponseWriter, req *http.Request, prefObj *models.Preference, user *models.User, provider models.Provider) {
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		msg := "unable to read request body"
		err = errors.Wrapf(err, msg)
		logrus.Error(err)
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
	perfTest := &SMPS.PerformanceTestConfig{}
	if err := protojson.Unmarshal(body, perfTest); err != nil {
		msg := "unable to parse the provided input"
		err = errors.Wrapf(err, msg)
		logrus.Error(err)
		http.Error(w, msg, http.StatusBadRequest)
		return
	}
	if err = models.SMPSPerformanceTestConfigValidator(perfTest); err != nil {
		logrus.Error(err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tid, err := provider.SMPSTestConfigStore(req, perfTest)
	if err != nil {
		logrus.Errorf("unable to save user preferences: %v", err)
		http.Error(w, "unable to save user preferences", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write([]byte(tid))
}

func (h *Handler) UserTestPreferenceGet(w http.ResponseWriter, req *http.Request, prefObj *models.Preference, user *models.User, provider models.Provider) {
	testUUID := req.URL.Query().Get("uuid")
	if testUUID == "" {
		testObj, err := provider.SMPSTestConfigFetchAll(req)
		if err != nil {
			logrus.Error("error fetching test configs")
			http.Error(w, "error fetching test configs", http.StatusInternalServerError)
			return
		}
		custTestObjs := []*custTestConf{}
		for _, tst := range testObj {
			custTestObjs = append(custTestObjs, &custTestConf{
				Val: tst,
			})
		}
		body, err := json.Marshal(&custTestObjs)
		if err != nil {
			logrus.Error("error reading database")
			http.Error(w, "error reading database", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(body)
	} else {
		testObj, err := provider.SMPSTestConfigFetch(req, testUUID)
		if err != nil {
			logrus.Error("error fetching test configs")
			http.Error(w, "error fetching test configs", http.StatusInternalServerError)
			return
		}
		if testObj == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		m := jsonpb.Marshaler{
			EmitDefaults: true,
		}
		if err := m.Marshal(w, testObj); err != nil {
			logrus.Error("error reading database: %v", err)
			http.Error(w, "error reading database", http.StatusInternalServerError)
			return
		}
	}
}

func (h *Handler) UserTestPreferenceDelete(w http.ResponseWriter, req *http.Request, prefObj *models.Preference, user *models.User, provider models.Provider) {
	testUUID := req.URL.Query().Get("uuid")
	if testUUID == "" {
		logrus.Error("field uuid not found")
		http.Error(w, "field uuid not found", http.StatusBadRequest)
		return
	}
	provider.SMPSTestConfigDelete(req, testUUID)
}
