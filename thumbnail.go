package main

import (
	"bytes"
	"github.com/nfnt/resize"
	"image"
	"image/jpeg"
	//	"log"
	"os"
)

func makeJPEGThumbnail(path string, width uint, height uint) (*bytes.Buffer, error) {

	// data, err := ioutil.ReadFile(path)
	// if err != nil {
	// 	log.Printf("error reading %v. %v", path, err)
	// 	return nil, err
	// }

	//	image, format, err := image.Decode(ioutil.NewReader(data))
	fileReader, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	image, _, err := image.Decode(fileReader)
	//log.Printf("Image format %v\n", format)
	if err != nil {
		//log.Printf("error decoding %v\n", err)
		return nil, err
	}

	thumbnailImage := resize.Resize(width, height, image, resize.NearestNeighbor)

	//var thumbnailData bytes.Buffer
	thumbnailData := new(bytes.Buffer)
	err = jpeg.Encode(thumbnailData, thumbnailImage, nil)
	//return thumbnailData.Bytes(), err
	return thumbnailData, err
}
