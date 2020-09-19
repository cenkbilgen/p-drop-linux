package main

import (
	"fmt"
	"log"

	"encoding/json"
	"github.com/julienschmidt/httprouter"
	"net/http"
	//"net/http/httputil" // debugging

	"flag"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"math/rand"

	"strings"

	"github.com/skip2/go-qrcode"

	"github.com/grandcat/zeroconf"
	"net"

	"encoding/base64"

	"os/signal" // zeroconf captures and handles os signals
	"syscall"
)

// MARK: HTTP Routes

////////////////////////////////////////////////////////////////////
// MARK: Upload Handler
  // response to a recieve request from  client

type UploadBinaryHandler struct {}
func (h *UploadBinaryHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

	log.Printf("upload request %v\n", request)

	// -- start
	err := request.ParseMultipartForm(5>>20) //use 5 MB in memory, the rest in disk
	if err != nil {
		log.Printf("parse error, %v", err)
		return
	}
	// // debug
	// dump, _ := httputil.DumpRequest(request, true)
	// ioutil.WriteFile("form_data", dump, 0644)
	//
	multipartForm := request.MultipartForm

	// iterate all File Parts of Form
	fileParts := multipartForm.File
	log.Printf("Received %v File Parts\n", len(fileParts))

	for key, headers := range fileParts {
		log.Printf("-- key %v - headers %v\n", key, len(headers))
		for _, header := range headers {
			filename := header.Filename
			size := header.Size
			file, err := header.Open()
			if err != nil {
				log.Printf("Error saving %v (%v)- %v\n", filename, size, err)
				continue
			}
			defer file.Close()
			data, err := ioutil.ReadAll(file)
			ioutil.WriteFile(filename, data, 0644)
		}
	}

}

////////////////////////////////////////////////////////////////////
// MARK: Download Request
// Respond to a send request from device

// keep a map of fileID to a file paths
// to avoid having the client know anything about the local file paths
var filemap map[string]string
type DownloadBinaryHandler struct {}
func (h *DownloadBinaryHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

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

type AvailableDownloadsHandler struct {}
func (h *AvailableDownloadsHandler) ServeHTTP(response http.ResponseWriter, request *http.Request) {

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
			log.Printf("%v %v - %v %v\n", fileID, path, filename, size)

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

var configPath string

func main() {

	filemap = make(map[string]string)

	// -- Flags
	showQR := flag.Bool("qr", false, "when sending, use QR code to connect devices (if automatic discovery is not working)")
	port := flag.Int("port", 3000, "listening network port")

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

	name := "transfer-" + randomString(5)
	service := "_p-drop._tcp"
	domain := "local."
	txt := []string{"txtver=1", "host=" + host, "port=" + portString, "path=" + strings.Join(fileKeys, fileKeysSeparator)}

	localInterfaces := []net.Interface{*iface}
	log.Printf("localInterfaces: %v\n", localInterfaces)
	server, err = zeroconf.Register(name, service, domain, *port, txt, localInterfaces)
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

	// WITH GZIP
	// router.Handler(http.MethodPost, "/upload_binary", Gzip(&UploadBinaryHandler{}))
	// router.Handler(http.MethodGet, "/download_binary", Gzip(&DownloadBinaryHandler{}))
	// router.Handler(http.MethodGet, "/available_downloads", Gzip(&AvailableDownloadsHandler{}))
	// WITH Brotli
	// router.Handler(http.MethodPost, "/upload_binary", Brotli(&UploadBinaryHandler{}))
	// router.Handler(http.MethodGet, "/download_binary", Brotli(&DownloadBinaryHandler{}))
	// router.Handler(http.MethodGet, "/available_downloads", Brotli(&AvailableDownloadsHandler{}))
	// RAW
	router.Handler(http.MethodPost, "/upload_binary", &UploadBinaryHandler{})
	router.Handler(http.MethodGet, "/download_binary", &DownloadBinaryHandler{})
	router.Handler(http.MethodGet, "/available_downloads", &AvailableDownloadsHandler{})

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

	go func() {
		err = http.ListenAndServeTLS(":"+strconv.Itoa(*port), pub_key, priv_key, router)
		check_error(err, true)
	}()

	// MARK: Signal Handler
	signalChannel := make(chan os.Signal, 1)
	signal.Notify(signalChannel, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM)

	select {
		case <-signalChannel:
	}

	log.Printf("Shutting down")

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

const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randomString(n int) string {
	alphabetN := len(alphabet)
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(alphabetN)]
	}
	return string(b)
}
