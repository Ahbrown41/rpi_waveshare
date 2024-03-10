// Package epd7in5 is an interface for the Waveshare 7.5inch e-paper display (wiki).
//
// The GPIO and SPI communication is handled by the awesome Periph.io package; no CGO or other dependecy needed.
//
// Tested on Raspberry Pi 3B / 3B+ / 4B with Raspbian Stretch.
//
// For more information please check the examples and doc folders.
package epd7in5

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"log"
	"periph.io/x/conn/v3"
	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
	"time"
)

const (
	Epd7in5V2Width  = 800
	Epd7in5V2Height = 480
)

const (
	PanelSetting            byte = 0x00
	PowerSetting            byte = 0x01
	PowerOff                byte = 0x02
	PowerOffSequenceSetting byte = 0x03
	DeepSleep               byte = 0x07
	DataStartTransmission1  byte = 0x10
	DisplayRefresh          byte = 0x12
	AutoMeasurementVcom     byte = 0x80
)

// Epd is a handle to the display controller.
type Epd struct {
	c          conn.Conn
	dc         gpio.PinOut
	cs         gpio.PinOut
	rst        gpio.PinOut
	busy       gpio.PinIO
	widthByte  int
	heightByte int
}

// New returns a Epd object that communicates over SPI to the display controller.
func New(dcPin, csPin, rstPin, busyPin string) (*Epd, error) {
	if _, err := host.Init(); err != nil {
		return nil, err
	}

	// DC pin
	dc := gpioreg.ByName(dcPin)
	if dc == nil {
		return nil, errors.New("spi: failed to find DC pin")
	}

	if dc == gpio.INVALID {
		return nil, errors.New("epd: use nil for dc to use 3-wire mode, do not use gpio.INVALID")
	}

	if err := dc.Out(gpio.Low); err != nil {
		return nil, err
	}

	// CS pin
	cs := gpioreg.ByName(csPin)
	if cs == nil {
		return nil, errors.New("spi: failed to find CS pin")
	}

	if err := cs.Out(gpio.Low); err != nil {
		return nil, err
	}

	// RST pin
	rst := gpioreg.ByName(rstPin)
	if rst == nil {
		return nil, errors.New("spi: failed to find RST pin")
	}

	if err := rst.Out(gpio.Low); err != nil {
		return nil, err
	}

	// BUSY pin
	busy := gpioreg.ByName(busyPin)
	if busy == nil {
		return nil, errors.New("spi: failed to find BUSY pin")
	}

	if err := busy.In(gpio.PullDown, gpio.RisingEdge); err != nil {
		return nil, err
	}

	// SPI
	port, err := spireg.Open("")
	if err != nil {
		return nil, err
	}

	c, err := port.Connect(5*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		if err := port.Close(); err != nil {
			return nil, err
		}
		return nil, err
	}

	var widthByte, heightByte int

	widthByte = Epd7in5V2Width / 8
	heightByte = Epd7in5V2Height

	e := &Epd{
		c:          c,
		dc:         dc,
		cs:         cs,
		rst:        rst,
		busy:       busy,
		widthByte:  widthByte,
		heightByte: heightByte,
	}

	return e, nil
}

// Reset can be also used to awaken the device.
func (e *Epd) Reset() error {
	if err := e.rst.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := e.rst.Out(gpio.Low); err != nil {
		return err
	}
	time.Sleep(2 * time.Millisecond)
	if err := e.rst.Out(gpio.High); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	return nil
}

/**
 * Send Command
 */
func (e *Epd) sendCommand(cmd byte) error {
	if err := e.dc.Out(gpio.Low); err != nil {
		return err
	}
	if err := e.cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := e.c.Tx([]byte{cmd}, nil); err != nil {
		return err
	}
	if err := e.cs.Out(gpio.High); err != nil {
		return err
	}
	return nil
}

/**
 * Write Data
 */
func (e *Epd) sendData(data byte) error {
	if err := e.dc.Out(gpio.High); err != nil {
		return err
	}
	if err := e.cs.Out(gpio.Low); err != nil {
		return err
	}
	if err := e.c.Tx([]byte{data}, nil); err != nil {
		return err
	}
	if err := e.cs.Out(gpio.High); err != nil {
		return err
	}
	return nil
}

// WaitUntilIdle waits until the display is idle.
func (e *Epd) waitUntilIdle() {
	log.Println("e-paper busy")
	for e.busy.Read() == gpio.Low {
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(5 * time.Millisecond)
	log.Println("e-paper busy release")
}

// TurnOnDisplay Turns on the display.
func (e *Epd) turnOnDisplay() error {
	if err := e.sendCommand(DisplayRefresh); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()
	return nil
}

// Init initializes the display config.
// It should be only used when you put the device to sleep and need to re-init the device.
func (e *Epd) Init() error {
	log.Println("e-paper init")
	if err := e.Reset(); err != nil {
		return err
	}

	if err := e.sendCommand(PowerSetting); err != nil {
		return err
	} //POWER SETTING

	if err := e.sendData(0x07); err != nil {
		return err
	}
	if err := e.sendData(0x07); err != nil {
		return err
	} //VGH=20V,VGL=-20V
	if err := e.sendData(0x3f); err != nil {
		return err
	} //VDH=15V
	if err := e.sendData(0x3f); err != nil {
		return err
	} //VDL=-15V

	if err := e.sendCommand(0x06); err != nil {
		return err
	}
	if err := e.sendData(0x17); err != nil {
		return err
	}
	if err := e.sendData(0x17); err != nil {
		return err
	}
	if err := e.sendData(0x28); err != nil {
		return err
	}
	if err := e.sendData(0x17); err != nil {
		return err
	}

	if err := e.sendCommand(0x04); err != nil {
		return err
	} //POWER ON
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()

	if err := e.sendCommand(0x00); err != nil {
		return err
	} //PANNEL SETTING
	if err := e.sendData(0x1F); err != nil {
		return err
	} //KW-3f   KWR-2F	BWROTP 0f	BWOTP 1f

	if err := e.sendCommand(0x61); err != nil {
		return err
	}
	if err := e.sendData(0x03); err != nil {
		return err
	}
	if err := e.sendData(0x20); err != nil {
		return err
	}
	if err := e.sendData(0x01); err != nil {
		return err
	}
	if err := e.sendData(0xE0); err != nil {
		return err
	}

	if err := e.sendCommand(0x15); err != nil {
		return err
	}
	if err := e.sendData(0x00); err != nil {
		return err
	}

	if err := e.sendCommand(0x50); err != nil {
		return err
	}
	if err := e.sendData(0x10); err != nil {
		return err
	}
	if err := e.sendData(0x07); err != nil {
		return err
	}

	if err := e.sendCommand(0x60); err != nil {
		return err
	}
	if err := e.sendData(0x22); err != nil {
		return err
	}
	return nil
}

func (e *Epd) InitFast() error {
	if err := e.Reset(); err != nil {
		return err
	}
	if err := e.sendCommand(0x00); err != nil {
		return err
	}
	if err := e.sendData(0x1F); err != nil {
		return err
	}

	if err := e.sendCommand(0x50); err != nil {
		return err
	}
	if err := e.sendData(0x10); err != nil {
		return err
	}
	if err := e.sendData(0x07); err != nil {
		return err
	}

	if err := e.sendCommand(0x04); err != nil {
		return err
	} //POWER ON
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()

	if err := e.sendCommand(0x06); err != nil {
		return err
	}
	if err := e.sendData(0x27); err != nil {
		return err
	}
	if err := e.sendData(0x27); err != nil {
		return err
	}
	if err := e.sendData(0x18); err != nil {
		return err
	}
	if err := e.sendData(0x17); err != nil {
		return err
	}

	if err := e.sendCommand(0xE0); err != nil {
		return err
	}
	if err := e.sendData(0x02); err != nil {
		return err
	}
	if err := e.sendData(0xE5); err != nil {
		return err
	}
	if err := e.sendData(0x5A); err != nil {
		return err
	}

	return nil
}

func (e *Epd) InitPart() error {
	if err := e.Reset(); err != nil {
		return err
	}
	if err := e.sendCommand(0x00); err != nil {
		return err
	}
	if err := e.sendData(0x1F); err != nil {
		return err
	}

	if err := e.sendCommand(0x04); err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	e.waitUntilIdle()

	if err := e.sendCommand(0xE0); err != nil {
		return err
	}
	if err := e.sendData(0x02); err != nil {
		return err
	}
	if err := e.sendCommand(0xE5); err != nil {
		return err
	}
	if err := e.sendData(0x6E); err != nil {
		return err
	}
	return nil
}

// Clear clears the screen.
func (e *Epd) Clear() error {
	if err := e.sendCommand(DataStartTransmission1); err != nil {
		return err
	}

	for j := 0; j < e.heightByte; j++ {
		for i := 0; i < e.widthByte; i++ {
			for k := 0; k < 4; k++ {
				if err := e.sendData(0x33); err != nil {
					return err
				}
			}
		}
	}

	if err := e.turnOnDisplay(); err != nil {
		return err
	}
	return nil
}

// Display takes a byte buffer and updates the screen.
func (e *Epd) Display(img []byte) error {
	log.Println("Start e-paper display")
	if err := e.sendCommand(DataStartTransmission1); err != nil {
		return err
	}

	for j := 0; j < e.heightByte; j++ {
		for i := 0; i < e.widthByte; i++ {
			dataBlack := ^img[i+j*e.widthByte]

			for k := 0; k < 8; k++ {
				var data byte

				if dataBlack&AutoMeasurementVcom > 0 {
					data = PanelSetting
				} else {
					data = PowerOffSequenceSetting
				}

				data <<= 4
				dataBlack <<= 1
				k++

				if dataBlack&AutoMeasurementVcom > 0 {
					data |= PanelSetting
				} else {
					data |= PowerOffSequenceSetting
				}

				dataBlack <<= 1

				if err := e.sendData(data); err != nil {
					return err
				}
			}
		}
	}
	log.Println("End e-paper display image process")
	if err := e.turnOnDisplay(); err != nil {
		return err
	}
	log.Println("End e-paper display")
	return nil
}

// Sleep puts the display in power-saving mode.
// You can use Reset() to awaken and Init() to re-initialize the display.
func (e *Epd) Sleep() error {
	if err := e.sendCommand(PowerOff); err != nil {
		return err
	}
	e.waitUntilIdle()
	if err := e.sendCommand(DeepSleep); err != nil {
		return err
	}
	if err := e.sendData(0xA5); err != nil {
		return err
	}
	return nil
}

// Convert converts the input image into a ready-to-display byte buffer.
func (e *Epd) Convert(img image.Image) []byte {
	var byteToSend byte = PanelSetting
	var bgColor = 1

	buffer := bytes.Repeat([]byte{PanelSetting}, e.widthByte*e.heightByte)

	for j := 0; j < Epd7in5V2Height; j++ {
		for i := 0; i < Epd7in5V2Width; i++ {
			bit := bgColor

			if i < img.Bounds().Dx() && j < img.Bounds().Dy() {
				bit = color.Palette([]color.Color{color.Black, color.White}).Index(img.At(i, j))
			}

			if bit == 1 {
				byteToSend |= AutoMeasurementVcom >> (uint32(i) % 8)
			}

			if i%8 == 7 {
				buffer[(i/8)+(j*e.widthByte)] = byteToSend
				byteToSend = PanelSetting
			}
		}
	}

	return buffer
}
