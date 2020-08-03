package main

import (
	"encoding/json"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"log"
	"net/http"
	//"net/http/httputil" // debugging
	"flag"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"

	"github.com/grandcat/zeroconf"
	"net"

	"encoding/base64"

	"os/signal" // zeroconf captures and handles os signals
	"syscall"
	//	"time"
)

// MARK: HTTP Routes

////////////////////////////////////////////////////////////////////
// MARK: Upload Handler
// response to a recieve request from client

func UploadBinaryHandler(response http.ResponseWriter, request *http.Request, _ httprouter.Params) {

	// log.Printf("upload request %v\n", request)

	// -- start
	err := request.ParseMultipartForm(100000) //use 1 MB in memory, the rest in disk
	if err != nil {
		log.Printf("parse error, %v", err)
		return
	}

	multipartForm := request.MultipartForm
	//log.Printf("Form %v\n", multipartForm)

	fileParts := multipartForm.File // File map[string][]*FileHeader

	//valueParts := multipartForm.Value // Value map[string][]string
	// for n, valuePart := range valueParts {
	// 	for m, value := range valuePart {
	// 		log.Printf("value part %v. %v: %v\n", n, m, value)
	// 	}
	// }

	// iterate all File Parts of Form

	for n, filePart := range fileParts {
		//log.Printf("file part %v. has %v file headers\n", n, len(headers))
		for m, header := range filePart {
			filename := header.Filename
			size := header.Size
			//	log.Printf("file part %v. header %v. %v (%v %v)\n", n, m, header, filename, size)
			log.Printf("receiving %v (%v)\n", filename, size)

			file, err := header.Open() //first check len(headers) is correct  // io.Reader?
			if err != nil {
				log.Printf("error part %v %v: %v", n, m, err)
				continue
			}

			defer file.Close()

			data, err := ioutil.ReadAll(file)

			ioutil.WriteFile(filename, data, 0644)

			// sum := md5.New().Sum(file)
			// log.Printf("md5: %x\n", sum)
		}

	}

}

////////////////////////////////////////////////////////////////////
// MARK: Download Request
// Respond to a send request from device

// keep a map of fileID to a file paths
// to avoid having the client know anything about the local file paths
var filemap map[string]string

func DownloadBinaryHandler(response http.ResponseWriter, request *http.Request, _ httprouter.Params) {

	fileID := request.URL.Query().Get("fileID")
	if len(fileID) == 0 {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	path, exists := filemap[fileID]
	if exists == false {
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	data, err := ioutil.ReadFile(path)
	if err != nil {
		log.Printf("error reading %v. %v", path, err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	size := len(data)

	response.Header().Add("Content-Length", strconv.Itoa(size))

	_, filename := filepath.Split(path)
	response.Header().Add("Content-Disposition", "filename="+filename)

	log.Printf("sending %v (%v)\n", filename, size)

	writeSize, err := response.Write(data)
	if err != nil {
		log.Printf("error responding with data. %v\n", err)
	}

	// check
	if writeSize != size {
		log.Printf("problem writing file, expected %v, wrote %v\n", size, writeSize)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

}

// MARK: Available Downloads
// Sends list of files availabele to be sent, and thumbnail if jpeg

type AvailableDownload struct {
	DeviceID string `json:"deviceID"` // 0 for broadcast
	FileID   string `json:"fileID"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Preview  string `json:"preview"`
	Type     string `json:"type"`
	Valid    bool   `json:"valid"`
}

var availableDownloads []AvailableDownload

func AvailableDownloadsHandler(response http.ResponseWriter, request *http.Request, _ httprouter.Params) {

	respond := func(response http.ResponseWriter, request *http.Request, status int, err error) {
		response.Header().Add("Content-Type", "application/json")
		if err != nil {
			response.WriteHeader(status)
			message := fmt.Sprintf("{\"error\": \"%v\"}", err)
			io.WriteString(response, message)
		}
	}

	fileCount := 0

	for fileID, path := range filemap {
		fileInfo, err := os.Stat(path)
		if os.IsNotExist(err) {
			//fmt.Printf("%v does not exist. marking as invalid\n", path)
			availableDownload := AvailableDownload{FileID: fileID, Name: "-", Size: 0, DeviceID: "0", Type: "-", Valid: false}
			availableDownloads[fileCount] = availableDownload

		} else {
			_, filename := filepath.Split(path)
			size := fileInfo.Size()
			//log.Printf("%v %v - %v %v\n", fileID, path, filename, size)

			//mimetype
			data, err := ioutil.ReadFile(path)
			if err != nil {
				log.Printf("error reading %v. %v", path, err)
				response.WriteHeader(http.StatusInternalServerError)
				return
			}
			mimetype := http.DetectContentType(data)
			//log.Printf("%v - %v\n", filename, mimetype)
			preview := ""
			if mimetype == "image/jpeg" {
				previewBuffer, err := makeJPEGThumbnail(path, 120, 0)
				if err != nil {
					//		log.Printf("Thumbnail error %v, %v\n", filename, err)
				} else {
					preview = base64.StdEncoding.EncodeToString(previewBuffer.Bytes())
				}
			}
			availableDownload := AvailableDownload{FileID: fileID, Name: filename, Size: size, DeviceID: "0", Type: mimetype, Preview: preview, Valid: true}
			availableDownloads[fileCount] = availableDownload
		}

		fileCount += 1
	}

	encoder := json.NewEncoder(response)
	encoder.Encode(availableDownloads[0:fileCount])

	respond(response, request, http.StatusOK, nil)

}

// // MARK: Device Update ====================================
// Use for dealing with remote notifications when files available for receiving

// type DeviceUpdateRequest struct {
// 	DeviceID       string `json:"deviceID"`
// 	NotificationID string `json:"notificationID"`
// 	Name           string `json:"name"`
// }
//
// //
// func DeviceHandler(response http.ResponseWriter, request *http.Request, _ httprouter.Params) {
//
// 	var update DeviceUpdateRequest
//
// 	decoder := json.NewDecoder(request.Body)
// 	err := decoder.Decode(&update)
// 	check_error(err, false)
//
// 	filePath := configPath + "/" + update.DeviceID
//
// 	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
// 	if err != nil {
// 		log.Printf("error opening/creating config file for %v. %v", filePath, err)
// 		response.WriteHeader(http.StatusInternalServerError)
// 		return
// 	}
//
// 	defer file.Close()
//
// 	_, err = file.WriteString(update.Name + "\n" + update.NotificationID)
// 	if err != nil {
// 		log.Printf("error writing config file for %v. %v", filePath, err)
// 		response.WriteHeader(http.StatusInternalServerError)
// 		return
// 	}
//
// 	response.WriteHeader(http.StatusCreated)
//
// }

// ------- Main()

// var notify_chan = make(chan *apns2.Notification, 300)
// var resp_chan = make(chan *apns2.Response, 300)

var configPath string

func main() {

	filemap = make(map[string]string)

	// -- Flags
	// upload := flag.Bool("send", false, "files to send to device")
	// download := flag.Bool("receive", false, "wait to receive files from device")
	showQR := flag.Bool("qr", false, "when sending, use QR code to connect devices (if automatic discovery is not working)")
	port := flag.Int("port", 3000, "listening network port")
	//setup := flag.Bool("setup", false, "setup new iOS device for notifications")

	flag.Parse()

	var server *zeroconf.Server

	if len(flag.Args()) > 0 { // files specified for uploading

		// remaining arguments should be file paths to upload
		for n, path := range flag.Args() {

			// if file exists, add to the file map
			_, err := os.Stat(path)
			if os.IsNotExist(err) {
				fmt.Printf("%v does not exist. skipping\n", path)
			} else {
				key := strconv.Itoa(n) // for now just key is the index
				filemap[key] = path
			}

		}

	}

	availableDownloads = make([]AvailableDownload, len(filemap))

	//url := "app-p-drop://download?host=" + host + "&port=" + strconv.Itoa(*port) + "&files=1"
	iface, ip4, err := localIP4()
	check_error(err, true)
	host := ip4.String()
	log.Printf("interface %v - %v\n", iface, ip4)

	appURL, _ := url.Parse("app-p-drop://download/")
	parameters := url.Values{}
	parameters.Add("host", host)
	portString := strconv.Itoa(*port)
	parameters.Add("port", portString)
	var fileKeys []string
	for fileKey, _ := range filemap {
		fileKeys = append(fileKeys, fileKey)
	}
	fileKeysSeparator := "_||_"
	parameters.Add("files", strings.Join(fileKeys, fileKeysSeparator)) // encode filemap IDs as _||_ separated
	appURL.RawQuery = parameters.Encode()

	// Start Bonjour/ZeroConf Service Registration

	name := "p-drop"
	service := "_p-drop._tcp"
	domain := "local."
	txt := []string{"txtver=1", "host=" + host, "port=" + portString, "path=" + strings.Join(fileKeys, fileKeysSeparator)}

	localInterfaces := []net.Interface{*iface}
	log.Printf("localInterfaces: %v\n", localInterfaces)
	server, err = zeroconf.Register(name, service, domain, *port, txt, localInterfaces)
	//server, err = zeroconf.Register(name, service, domain, 3000, []string{"txtver=1", "p=12345"}, nil)
	if err != nil {
		panic(err)
	}

	defer server.Shutdown() // not called in SignalHandler

	// QR Code

	if *showQR {
		qrcode, err := qrcode.New(appURL.String(), qrcode.Medium)
		check_error(err, false)

		fmt.Printf("\n%v\n", qrcode.ToSmallString(false))
		fmt.Printf("%v\n", appURL.String())
	}

	filemapCount := len(filemap)
	if filemapCount > 0 {
		fmt.Printf("Ready to send files (%v) and ", filemapCount)
	}
	fmt.Printf("Waiting to recieve files...\n")

	// -- Router

	router := httprouter.New()

	router.POST("/upload_binary", UploadBinaryHandler)
	router.GET("/download_binary", DownloadBinaryHandler)
	router.GET("/available_downloads", AvailableDownloadsHandler)
	// router.POST("/device", DeviceHandler) // to exchange

	// set up directories if not setup
	// TODO: Linux specific, use path packages

	// Configuration

	homePath := os.Getenv("HOME")
	configPath = homePath + "/.p-drop"

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		fmt.Println("Creating config path %v.", configPath)
		os.Mkdir(configPath, 0770)
	}

	// Key-pair

	priv_key := configPath + "/server.key"
	pub_key := configPath + "/server.crt"

	if _, err := os.Stat(priv_key); os.IsNotExist(err) {
		fmt.Println("Creating TLS key pair.")
		generate_cert(configPath, "server")
	}

	//go func() {
	err = http.ListenAndServeTLS(":"+strconv.Itoa(*port), pub_key, priv_key, router)
	check_error(err, true)
	//}()

	// fmt.Printf("Waiting for signals\n")
	//  s := <-signal_chan
	//  fmt.Printf("Received signal: %v\n", s)

	// MARK: Signal Handler
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)
	//go handleSignal(signalChannel, server)

	log.Printf("Listening for singals.")
	// blocks
	s := <-signalChannel

	log.Printf("Got signal %v. Shutting down\n", s)
	server.Shutdown()

	// select {
	// case s:= <-signalChannel:
	// 	log.Printf("recevied %v\n", s)
	// default:
	// 	log.Printf("received nothing yet")
	// }

}

// MARK: Signal Handler

// NOTE: Blocking
// NOTE: NOT USED ///////
func handleSignal(signalChannel <-chan os.Signal, server *zeroconf.Server) {
	fmt.Printf("Waiting for signals\n")

	select {
	case s := <-signalChannel:
		fmt.Printf("Received signal: %v\n", s)
		switch s {
		case syscall.SIGTERM, syscall.SIGHUP:
			fmt.Printf("Shutting down.\n")
			server.Shutdown()
			os.Exit(0) // defer is not called on Exit
		default:
			fmt.Printf("Not Handling.\n")
		}
	default:
		fmt.Printf("No signal received.\n")
	}

}

// MARK: Utility Functions

func check_error(err error, fatal bool) {
	if err != nil && fatal {
		log.Fatal(err)
	} else if err != nil {
		log.Println(err)
	}
}

func check_error_message(err error, fatal bool, message string) bool {
	if err != nil && fatal {
		log.Fatal(err)
		return true
	} else if err != nil {
		log.Printf("%v: %v\n", message, err)
		return true
	}
	return false
}
