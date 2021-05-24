package main

// #cgo CFLAGS: -I.
// #cgo LDFLAGS: -L. -l:libMtcControlLib.so.1
// #include <stdlib.h>
// #include "mtclib.h"
import "C"

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"path/filepath"
	"strings"
	"unsafe"
)

type (
	MTC struct {
		dev unsafe.Pointer
	}

	Devices struct {
		Videos  []string
		Serials []string
	}

	GetFrameResult struct {
		Image []byte
		Data  string
	}
)

func (m *MTC) getDev() unsafe.Pointer {
	m.Init(nil, nil)
	return m.dev
}

func (m *MTC) Init(_ *int, success *bool) error {
	if m.dev == nil {
		m.dev = C.init()
	}
	if success != nil {
		*success = true
	}
	return nil
}

func (m *MTC) Deinit(_ *int, success *bool) error {
	if m.dev != nil {
		C.deInit(m.dev)
		m.dev = nil
		*success = true
	}
	return nil
}

func (m *MTC) ConnectCamera(_name *string, success *bool) error {
	name := C.CString(*_name)
	defer C.free(unsafe.Pointer(name))
	C.disconnectCamera(m.getDev())
	ret := int(C.connectCamera(m.getDev(), name))
	if ret == 0 {
		*success = true
		return nil
	} else if ret == 1 {
		*success = false
		return nil
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
}

func (m *MTC) ConnectSerial(_port *string, success *bool) error {
	port := C.CString(*_port)
	defer C.free(unsafe.Pointer(port))
	ret := int(C.connectSerial(m.getDev(), port))
	if ret == 0 {
		*success = true
		return nil
	} else if ret == 1 {
		*success = false
		return nil
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
}

func (m *MTC) EnumDevice(_ *int, devices *Devices) error {
	videoDevPtr, videoDevSize, freeVideoDev, getVideoDev := makeString(1024)
	defer freeVideoDev()
	serialPtr, serialSize, freeSerial, getSerial := makeString(255)
	defer freeSerial()
	C.enumDevice(videoDevPtr, C.int(videoDevSize), serialPtr, C.int(serialSize))
	split := func(input string) []string {
		if input == "" {
			return []string{}
		}
		return strings.Split(input, "|")
	}
	videos := split(getVideoDev())
	matches, _ := filepath.Glob("/dev/video*")
	for _, m := range matches {
		exists := false
		for _, v := range videos {
			if v == m {
				exists = true
			}
		}
		if !exists {
			videos = append(videos, m)
		}
	}

	*devices = Devices{
		Videos:  videos,
		Serials: split(getSerial()),
	}
	return nil
}

func (m *MTC) GetDevModel(_ *int, model *string) error {
	resultPtr, resultSize, freeResult, getResult := makeString(128)
	defer freeResult()
	ret := int(C.getDevModel(m.getDev(), resultPtr, resultSize))
	if ret == 0 {
		*model = getResult()
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
	return nil
}

func (m *MTC) GetDevSn(_ *int, sn *string) error {
	resultPtr, resultSize, freeResult, getResult := makeString(13)
	defer freeResult()
	ret := int(C.getDevSn(m.getDev(), 1, resultPtr, resultSize))
	if ret == 0 {
		*sn = getResult()
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
	return nil
}

func (m *MTC) GetFrame(_ *int, result *GetFrameResult) error {
	width := int(C.getFrameWidth(m.getDev()))
	height := int(C.getFrameHeight(m.getDev()))

	imageSize := width * height * 4
	imagePtr := C.malloc(C.sizeof_char * C.ulong(imageSize))
	defer C.free(unsafe.Pointer(imagePtr))

	resultPtr, resultSize, freeResult, getResult := makeString(1024)
	defer freeResult()

	ret := int(C.getFrame(m.getDev(), (*C.char)(imagePtr), C.int(imageSize), resultPtr, C.int(resultSize)))
	if ret == 1 {
		return errors.New("ERR_INSUFFICIENT_IMAGE_BUFFER_SIZE")
	} else if ret == 2 {
		return errors.New("ERR_INSUFFICIENT_RESULT_BUFFER_SIZE")
	} else if ret == -1 {
		return errors.New("ERR_NO_NEW_IMAGE") // 还没有新图像
	} else if ret != 0 {
		return errors.New("ERR_UNKNOWN") // 未知
	}

	data := C.GoBytes(imagePtr, C.int(imageSize))
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			i := y*4*width + x*4
			img.SetRGBA(x, y, color.RGBA{
				R: data[i+2],
				G: data[i+1],
				B: data[i+0],
				A: data[i+3],
			})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 60}); err != nil {
		return err
	}
	*result = GetFrameResult{
		Image: buf.Bytes(),
		Data:  getResult(),
	}
	return nil
}

func (m *MTC) GetFrameHeight(_ *int, height *int) error {
	*height = int(C.getFrameHeight(m.getDev()))
	return nil
}

func (m *MTC) GetFrameWidth(_ *int, width *int) error {
	*width = int(C.getFrameWidth(m.getDev()))
	return nil
}

func (m *MTC) GetReoConfig(_ *int, json *string) error {
	resultPtr, resultSize, freeResult, getResult := makeString(2048)
	defer freeResult()
	ret := int(C.getReoconfig(m.getDev(), 0, resultPtr, resultSize))
	if ret == 0 {
		*json = getResult()
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
	return nil
}

func (m *MTC) GetSysVer(_ *int, info *map[string]string) error {
	verPtr, verSize, freeVer, getVer := makeString(128)
	defer freeVer()
	ret := int(C.getSysVer(m.getDev(), verPtr, verSize))
	if ret == 0 {
		str := strings.TrimSpace(getVer())
		lines := strings.Split(str, "\n")
		m := map[string]string{}
		for _, line := range lines {
			parts := strings.SplitN(line, "=", 2)
			m[strings.ToLower(parts[0])] = parts[1]
		}
		*info = m
		return nil
	} else if ret == 1 || ret == 2 {
		return errors.New("ERR_QUERY_FAILED") // 查询失败
	} else if ret == -2 {
		return errors.New("ERR_INSUFFICIENT_SIZE") // 长度不够
	} else {
		return errors.New("ERR_UNKNOWN") // 未知
	}
}

func makeString(size int) (pointer *C.char, uintsize C.uint, free func(), get func() string) {
	uintsize = C.uint(size)
	str := C.malloc(C.sizeof_char * C.ulong(uintsize))
	pointer = (*C.char)(str)
	free = func() {
		C.free(unsafe.Pointer(str))
	}
	get = func() string {
		b := C.GoBytes(str, C.int(size))
		n := bytes.IndexByte(b, 0)
		if n == -1 {
			return string(b)
		}
		return string(b[:n])
	}
	return
}
