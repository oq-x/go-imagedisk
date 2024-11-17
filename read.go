package imd

import (
	"errors"
	"io"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

type Header string

func (h Header) Version() string {
	return string(h[4:8])
}

func (h Header) Time() (time.Time, error) {
	return time.Parse("02/01/2006 15:04:05", string(h[10:]))
}

type Track struct {
	ModeValue,
	Cylinder,
	Head,
	NumberOfSectors,
	SectorSize byte

	SectorNumberingMap,
	SectorCylinderMap,
	SectorHeadMap []byte

	SectorDataRecords [][]byte
}

type File struct {
	Header  Header
	Comment string

	Tracks []Track
}

func Decode(r io.Reader) (file File, err error) {
	var header [0x1D]byte
	if _, err := r.Read(header[:]); err != nil {
		return file, err
	}
	file.Header = Header(string(header[:]))
	if err := validateHeader(file.Header); err != nil {
		return file, err
	}

	file.Comment, err = readStringASCIIEOF(r)
	if err != nil {
		return
	}

	for {
		modeValue, err := readByte(r)
		if err != nil {
			break
		}
		cylinder, err := readByte(r)
		if err != nil {
			return file, err
		}
		head, err := readByte(r)
		if err != nil {
			return file, err
		}
		numberOfSectors, err := readByte(r)
		if err != nil {
			return file, err
		}
		sectorSize, err := readByte(r)
		if err != nil {
			return file, err
		}

		sectorNumberingMap := make([]byte, numberOfSectors)
		if _, err := r.Read(sectorNumberingMap); err != nil {
			return file, err
		}

		var sectorCylinderMap, sectorHeadMap []byte

		if head&sectorCylinderMapMask != 0 {
			sectorCylinderMap = make([]byte, numberOfSectors)
			if _, err := r.Read(sectorCylinderMap); err != nil {
				return file, err
			}
		}

		if head&sectorHeadMapMask != 0 {
			sectorHeadMap = make([]byte, numberOfSectors)
			if _, err := r.Read(sectorHeadMap); err != nil {
				return file, err
			}
		}

		var sectorDataRecords = make([][]byte, numberOfSectors)

		var record byte
		for i := byte(0); i < numberOfSectors; i++ {
			if err := readBytePtr(r, &record); err != nil {
				return file, err
			}

			switch record {
			case 0: // unavailable
				continue
			case 1, 3, 5, 7: // regular sector data
				sectorDataRecords[i] = make([]byte, sectorSize)
				if _, err := r.Read(sectorDataRecords[i]); err != nil {
					return file, err
				}
			case 2, 4, 6, 8: // compressed (all bytes are the same)
				v, err := readByte(r)
				if err != nil {
					return file, err
				}
				sectorDataRecords[i] = make([]byte, sectorSize)
				fill(sectorDataRecords[i], v)
			}
		}

		file.Tracks = append(file.Tracks, Track{
			ModeValue:          modeValue,
			Cylinder:           cylinder,
			Head:               head,
			NumberOfSectors:    numberOfSectors,
			SectorSize:         sectorSize,
			SectorNumberingMap: sectorNumberingMap,
			SectorCylinderMap:  sectorCylinderMap,
			SectorHeadMap:      sectorHeadMap,
			SectorDataRecords:  sectorDataRecords,
		})
		break
	}

	return file, nil
}

func fill(dst []byte, v byte) {
	for i := 0; i < len(dst); i++ {
		dst[i] = v
	}
}

const (
	sectorCylinderMapMask = (1 << (iota + 6))
	sectorHeadMapMask
)

func readBytePtr(r io.Reader, dst *byte) error {
	_, err := r.Read(unsafe.Slice(dst, 1))

	return err
}

func readByte(r io.Reader) (byte, error) {
	var v byte
	err := readBytePtr(r, &v)

	return v, err
}

func readStringASCIIEOF(r io.Reader) (string, error) {
	var str string

	var byt [1]byte
	for {
		if _, err := r.Read(byt[:]); err != nil {
			return str, err
		}

		if byt[0] == 0x1A {
			return str, nil
		}

		str += string(byt[0])
	}
}

func validateHeader(input Header) error {
	if !strings.HasPrefix(string(input), "IMD ") {
		return errors.New("does not start with 'IMD '")
	}

	parts := strings.SplitN(string(input[4:]), ": ", 2)
	if len(parts) != 2 {
		return errors.New("missing ': ' separator")
	}

	version := parts[0]
	if len(version) < 4 || version[1] != '.' || len(version) > 6 {
		return errors.New("invalid version format")
	}
	if _, err := strconv.Atoi(version[:1]); err != nil {
		return errors.New("invalid major version number")
	}
	if _, err := strconv.Atoi(version[2:]); err != nil {
		return errors.New("invalid minor version number")
	}

	datetime := parts[1]
	if len(datetime) != 19 {
		return errors.New("invalid datetime length")
	}
	dateTimeParts := strings.Split(datetime, " ")
	if len(dateTimeParts) != 2 {
		return errors.New("datetime should contain a date and time separated by space")
	}

	date := dateTimeParts[0]
	if len(date) != 10 || date[2] != '/' || date[5] != '/' {
		return errors.New("invalid date format")
	}
	if _, err := time.Parse("02/01/2006", date); err != nil {
		return errors.New("invalid date values")
	}

	timeStr := dateTimeParts[1]
	if len(timeStr) != 8 || timeStr[2] != ':' || timeStr[5] != ':' {
		return errors.New("invalid time format")
	}
	if _, err := time.Parse("15:04:05", timeStr); err != nil {
		return errors.New("invalid time values")
	}

	return nil
}
