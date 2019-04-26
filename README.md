# p-drop-linux
Move files between iOS and Linux.

Transfer files without requiring a cloud service. Transfers are faster, easier and more secure because they are direct with nothing leaving the local network.
Files can also be left up and available for download to iOS at all times to anyone joining the WiFI network

### Requirements:

1. Both devices on the same WiFi network

2. On Linux, download `p-drop` from releases above (link: https://github.com/cenkbilgen/p-drop-linux/releases/download/v1.0/p-drop) or compile your own.

3. On iOS, the `p-drop` app

### Usage:

On your Linux shell run: `p-drop file1 file2` and the files will appear on the iOS app ready to download. 

At the same time you can select files on your iOS device to upload to Linux. 

If the iOS app does not see the files automatically, run with `-q` to generate a QR code on screen, which the iOS app can scan to get the info it needs.

### Compiling

Just run `make`. This will get the go dependencies, compile and link and you will end up with a `p-drop` binary file. 
The release version is made with `make dist`, it's just statically linked and stripped.
See the `Makefile`.

### How it works:

It's just a https server with a small API to list, send and recieve files. 
A unique keypair for TLS is generated the first time it's run.
The Linux service advertises itself using Bonjour/ZeroConf with multicast-DNS, which the iOS app picks up on to request a keyed list of what's available to download and how to upload.
Files are sent to the iOS device using base64 mime encoding multi-part form data and recieved from iOS as binary streams.




