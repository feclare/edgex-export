//
// Copyright (c) 2017 Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package client

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/drasko/edgex-export"
	"github.com/drasko/edgex-export/mongo"
	"github.com/go-zoo/bone"
	"go.uber.org/zap"
	"gopkg.in/mgo.v2/bson"
)

var registrationChanges chan bool

func getRegByID(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	id := bone.GetValue(r, "id")

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	reg := export.Registration{}
	if err := c.Find(bson.M{"id": id}).One(&reg); err != nil {
		logger.Error("Failed to query by id", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	res, err := json.Marshal(reg)
	if err != nil {
		logger.Error("Failed to query by id", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(res))
}

func getRegList(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	t := bone.GetValue(r, "type")

	var l string

	switch t {
	case "algorithms":
		l = `["None","Aes"]`
	case "compressions":
		l = `["None","Gzip","Zip"]`
	case "formats":
		l = `["JSON","XML","Serialized","IotCoreJSON","AzureJSON","CSV"]`
	case "destinations":
		l = `["DestMQTT", "TeDestZMQller", "DestIotCoreMQTT,
			"DestAzureMQTT", "DestRest"]`
	default:
		logger.Error("Unknown type: " + t)
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, "Unknown type: "+t)
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, l)
}

func getAllReg(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	reg := []export.Registration{}
	if err := c.Find(nil).All(&reg); err != nil {
		logger.Error("Failed to query all registrations", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	res, err := json.Marshal(reg)
	if err != nil {
		logger.Error("Failed to query all registrations", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(res))
}

func getRegByName(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	name := bone.GetValue(r, "name")

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	reg := export.Registration{}
	if err := c.Find(bson.M{"name": name}).One(&reg); err != nil {
		logger.Error("Failed to query by name", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	res, err := json.Marshal(reg)
	if err != nil {
		logger.Error("Failed to query by name", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	io.WriteString(w, string(res))
}

func addReg(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to query add registration", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}

	reg := export.Registration{}
	if err := json.Unmarshal(data, &reg); err != nil {
		logger.Error("Failed to query add registration", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	count, err := c.Find(bson.M{"name": reg.Name}).Count()
	if err != nil {
		logger.Error("Failed to query add registration", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}
	if count != 0 {
		logger.Error("Username already taken: " + reg.Name)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if err := c.Insert(reg); err != nil {
		logger.Error("Failed to query add registration", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	notifyRegistrationChange()
}

func updateReg(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		logger.Error("Failed to query update registration", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		io.WriteString(w, err.Error())
		return
	}

	var body map[string]interface{}
	if err := json.Unmarshal(data, &body); err != nil {
		logger.Error("Failed to query update registration", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
	}

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	name := body["name"]
	query := bson.M{"name": name}
	update := bson.M{"$set": body}

	if err := c.Update(query, update); err != nil {
		logger.Error("Failed to query update registration", zap.Error(err))
		w.WriteHeader(http.StatusNotFound)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	notifyRegistrationChange()
}

func delRegByID(w http.ResponseWriter, r *http.Request) {
	id := bone.GetValue(r, "id")

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	if err := c.Remove(bson.M{"id": id}); err != nil {
		logger.Error("Failed to query by id", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	notifyRegistrationChange()
}

func delRegByName(w http.ResponseWriter, r *http.Request) {
	name := bone.GetValue(r, "name")

	s := repo.Session.Copy()
	defer s.Close()
	c := s.DB(mongo.DBName).C(mongo.CollectionName)

	if err := c.Remove(bson.M{"name": name}); err != nil {
		logger.Error("Failed to query by name", zap.Error(err))
		w.WriteHeader(http.StatusInternalServerError)
		io.WriteString(w, err.Error())
		return
	}

	w.WriteHeader(http.StatusOK)
	notifyRegistrationChange()
}

func SetNotification(ch chan bool) {
	registrationChanges = ch
}

func notifyRegistrationChange() {
	if registrationChanges != nil {
		registrationChanges <- true
	}
}
