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
// - Event buffer management per sender(do not block distro.Loop on full
//   registration channel)

import (
	"time"

	"github.com/drasko/edgex-export"
	"github.com/drasko/edgex-export/mongo"
	"go.uber.org/zap"
)

func (reg *RegistrationInfo) update(newReg export.Registration) bool {
	reg.registration = newReg

	reg.format = nil
	switch newReg.Format {
	case export.FormatJSON:
		reg.format = jsonFormater{}
	case export.FormatXML:
		reg.format = xmlFormater{}
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

	reg.encrypt = nil
	switch newReg.Encryption.Algo {
	case export.EncNone:
		reg.encrypt = nil
	case export.EncAes:
		reg.encrypt = NewAESEncryption(newReg.Encryption)
	default:
		logger.Info("Encryption not supported: ", zap.String("Algorithm", newReg.Encryption.Algo))

	}

	reg.filter = nil
	if len(newReg.Filter.DeviceIDs) > 0 {
		reg.filter = NewDeviceIDFilter(newReg.Filter.DeviceIDs)
	}

	/*	if len(newReg.Filter.ValueDescriptorIDs) > 0 {
			reg.filter = NewValueDescFilter(newReg.Filter.ValueDescriptorIDs)
		}
	*/

	reg.chRegistration = make(chan *RegistrationInfo)
	reg.chEvent = make(chan *export.Event)

	return true
}

func (reg RegistrationInfo) processEvent(event *export.Event) {
	// Valid Event Filter, needed?

	// TODO Device filtering
	if reg.filter != nil {
		filtered := reg.filter.Filter(event)
		logger.Info("Event filtered")

		if !filtered {
			return
		}
	}
	// TODO Value filtering

	formated := reg.format.Format(event)

	compressed := formated
	if reg.compression != nil {
		compressed = reg.compression.Transform(formated)
	}

	encrypted := compressed
	if reg.encrypt != nil {
		encrypted = reg.encrypt.Transform(compressed)
	}

	logger.Info("Event: ", zap.Any("event", event))
	reg.sender.Send(encrypted)
}

func registrationLoop(reg RegistrationInfo) {
	logger.Info("registration loop started")
	for {
		select {
		case event := <-reg.chEvent:
			reg.processEvent(event)

		case newReg := <-reg.chRegistration:
			if newReg == nil {
				logger.Info("Terminate registration goroutine")
			} else {
				// TODO implement updating the registration info.
				logger.Info("Registration updated")
			}
		}
	}
}

// Loop - registration loop
func Loop(repo *mongo.Repository, errChan chan error) {

	var registrations []RegistrationInfo

	sourceReg := getRegistrations(repo)

	for i := range sourceReg {
		var reg RegistrationInfo
		if reg.update(sourceReg[i]) {
			registrations = append(registrations, reg)
			//logger.Info("Registration: ", zap.Any("reg", registrations), zap.Int("length", len(registrations)))
			go registrationLoop(reg)
		}
	}

	logger.Info("Starting registration loop")
	for {
		select {
		case e := <-errChan:
			// kill all registration goroutines
			for r := range registrations {
				registrations[r].chRegistration <- nil

			}
			logger.Info("exit msg", zap.Error(e))
			return

		case <-time.After(time.Second):
			// Simulate receiving events
			event := getNextEvent()
			//logger.Info("Event: ", zap.Any("event", event), zap.Int("length", len(registrations)))

			for r := range registrations {
				// TODO only sent event if it is not blocking
				registrations[r].chEvent <- event
			}
		}
	}
}
