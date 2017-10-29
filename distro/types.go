//
// (C) Copyright 2017
// Mainflux
//
// SPDX-License-Identifier:	Apache-2.0
//

package distro

import "github.com/drasko/edgex-export"

type Sender interface {
	Send(data string)
}

type Formater interface {
	Format( /*event*/ ) []byte
}

type Transformer interface {
	Transform(data []byte) []byte
}

type RegistrationInfo struct {
	registration export.Registration
	format       Formater
	compression  Transformer
	encrypt      Transformer
	sender       Sender
}
