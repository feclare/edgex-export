//
// Copyright (c) 2017
// Cavium
// Mainflux
//
// SPDX-License-Identifier: Apache-2.0
//

package distro

// TODO:
// - Filtering by id and value
// - Receive events from 0mq until a new message broker/rpc is chosen
// - Implement json/xml/.... serializers
// - Event buffer management per sender(do not block distro.Loop on full
//   registration channel)

import (
	"time"

	"github.com/drasko/edgex-export"
	"github.com/drasko/edgex-export/mongo"
	"go.uber.org/zap"
)

var registrationChanges chan bool

// To be removed when any other formater is implemented
type dummyFormat struct {
}

func (dummy dummyFormat) Format( /*event*/ ) []byte {
	return []byte("dummy")
}

var dummy dummyFormat

func newRegistrationInfo() *RegistrationInfo {
	reg := &RegistrationInfo{}

	reg.chRegistration = make(chan *export.Registration)
	reg.chEvent = make(chan bool)
	return reg
}

func (reg *RegistrationInfo) update(newReg export.Registration) bool {
	reg.registration = newReg

	reg.format = nil
	switch newReg.Format {
	case export.FormatJSON:
		// TODO reg.format = distro.NewJsonFormat()
		reg.format = dummy
	case export.FormatXML:
		// TODO reg.format = distro.NewXmlFormat()
		reg.format = dummy
	case export.FormatSerialized:
		// TODO reg.format = distro.NewSerializedFormat()
	case export.FormatIoTCoreJSON:
		// TODO reg.format = distro.NewIotCoreFormat()
	case export.FormatAzureJSON:
		// TODO reg.format = distro.NewAzureFormat()
	case export.FormatCSV:
		// TODO reg.format = distro.NewCsvFormat()
	default:
		logger.Info("Format not supported: ", zap.String("format", newReg.Format))
	}

	reg.compression = nil
	switch newReg.Compression {
	case export.CompNone:
		reg.compression = nil
	case export.CompGzip:
		reg.compression = gzipTransformer{}
	case export.CompZip:
		reg.compression = zlibTransformer{}
	default:
		logger.Info("Compression not supported: ", zap.String("compression", newReg.Compression))
	}

	reg.sender = nil
	switch newReg.Destination {
	case export.DestMQTT:
		reg.sender = NewMqttSender(newReg.Addressable)
	case export.DestZMQ:
		logger.Info("Destination ZMQ is not supported")
	case export.DestIotCoreMQTT:
		// TODO reg.sender = distro.NewIotCoreSender("TODO URL")
	case export.DestAzureMQTT:
		// TODO reg.sender = distro.NewAzureSender("TODO URL")
	case export.DestRest:
		reg.sender = NewHTTPSender(newReg.Addressable)
	default:
		logger.Info("Destination not supported: ", zap.String("destination", newReg.Destination))
	}
	if reg.format == nil || reg.sender == nil {
		logger.Error("Registration not supported")
		return false
	}

	return true
}

func (reg RegistrationInfo) processEvent( /*event*/ ) {
	// Valid Event Filter, needed?

	// TODO Device filtering

	// TODO Value filtering
	formated := reg.format.Format( /* event*/ )
	compressed := formated
	if reg.compression != nil {
		compressed = reg.compression.Transform(formated)
	}

	encrypted := compressed
	if reg.encrypt != nil {
		encrypted = reg.encrypt.Transform(compressed)
	}

	reg.sender.Send(encrypted)
	logger.Debug("Sent event with registration:",
		zap.String("Name", reg.registration.Name))
}

func registrationLoop(reg *RegistrationInfo) {
	logger.Info("registration loop started",
		zap.String("Name", reg.registration.Name))
	for {
		select {
		case /*event :=*/ <-reg.chEvent:
			reg.processEvent( /*event*/ )

		case newReg := <-reg.chRegistration:
			if newReg == nil {
				logger.Info("Terminating registration goroutine")
				return
			} else {
				if reg.update(*newReg) {
					logger.Info("Registration updated: OK",
						zap.String("Name", reg.registration.Name))
				} else {
					logger.Info("Registration updated: KO",
						zap.String("Name", reg.registration.Name))
					// TODO Something went wrong, need to remove this from
					// running registrations
				}
			}
		}
	}
}

func updateRunningRegistrations(running map[string]*RegistrationInfo,
	newRegistrations []export.Registration) {

	// kill all running registrations not in the new list
	for k, v := range running {
		toDelete := true
		for i := range newRegistrations {
			if v.registration.Name == newRegistrations[i].Name {
				toDelete = false
				break
			}
		}
		if toDelete {
			v.chRegistration <- nil
			delete(running, k)
		}
	}

	// Create or update registrations in the new list
	for i := range newRegistrations {
		v, found := running[newRegistrations[i].Name]
		if found {
			v.chRegistration <- &newRegistrations[i]
		} else {
			// Create new goroutine for this registration
			reg := newRegistrationInfo()
			if reg.update(newRegistrations[i]) {
				running[reg.registration.Name] = reg
				go registrationLoop(reg)
			}
		}
	}
}

// Loop - registration loop
func Loop(repo *mongo.Repository, errChan chan error) {

	registrations := make(map[string]*RegistrationInfo)

	updateRunningRegistrations(registrations, getRegistrations(repo))

	logger.Info("Starting registration loop")
	for {
		select {
		case e := <-errChan:
			// kill all registration goroutines
			for k, v := range registrations {
				v.chRegistration <- nil
				delete(registrations, k)
			}
			logger.Info("exit msg", zap.Error(e))
			return
		case <-registrationChanges:
			// kill all running registrations not in the new list
			logger.Info("Registration changes")
			updateRunningRegistrations(registrations, getRegistrations(repo))

		case <-time.After(time.Second):
			// Simulate receiving 10k events/seg
			for _, r := range registrations {
				// TODO only sent event if it is not blocking
				logger.Debug("sending event",
					zap.String("Name", r.registration.Name))
				r.chEvent <- true
			}
		}
	}
}

func SetNotification(ch chan bool) {
	registrationChanges = ch
}
