package cmd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"

	"github.com/Impyy/scytale"
	"github.com/Impyy/scytale/crypto"
	"github.com/spf13/cobra"
	"gopkg.in/h2non/filetype.v1"
)

type uploadFlags struct {
	Encrypt bool
	File    string
	Open    bool
	URL     string
}

var (
	uploadCmdFlags = new(uploadFlags)
	uploadCmd      = &cobra.Command{
		Use:   "ul",
		Short: "Upload a file",
		Long:  "Upload a file with optional encryption",
		Run:   startUpload,
	}
)

func init() {
	RootCmd.AddCommand(uploadCmd)
	uploadCmd.Flags().BoolVarP(&uploadCmdFlags.Encrypt, "encrypt", "e", false, "Encrypt the file before upload.")
	uploadCmd.Flags().StringVarP(&uploadCmdFlags.File, "file", "f", "-", "The file upload. Pass - to read from stdin.")
	uploadCmd.Flags().StringVarP(&uploadCmdFlags.URL, "url", "u", "", "The URL to send the upload request to.")
}

func startUpload(cmd *cobra.Command, args []string) {
	var keyString string
	encrypt := uploadCmdFlags.Encrypt
	req := &scytale.UploadRequest{IsEncrypted: encrypt}
	filename := uploadCmdFlags.File
	url := cfg.URL

	if uploadCmdFlags.URL != "" {
		url = uploadCmdFlags.URL
	}

	if filename == "-" {
		filename = "/dev/stdin"
	} else if !encrypt {
		req.Extension = path.Ext(filename)
	}

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		logger.Fatalf("read error: %s", err.Error())
	}
	if len(bytes) == 0 {
		logger.Fatalf("error: empty file")
		return
	}

	if req.Extension == "" {
		kind, _ := filetype.Match(bytes)
		if kind == filetype.Unknown {
			logger.Fatalln("error: unable to determine file type")
		}
		req.Extension = "." + kind.Extension
	}

	if encrypt {
		key, encryptedBytes, err := crypto.Encrypt(bytes)
		if err != nil {
			logger.Fatalf("encrypt error: %s\n", err.Error())
		}

		bytes = encryptedBytes
		keyString = base64.URLEncoding.EncodeToString(key[:])
	}

	req.Data = base64.StdEncoding.EncodeToString(bytes)
	res, err := uploadReq(url, req)
	if err != nil {
		logger.Fatalf("upload error: %s\n", err.Error())
	}

	loc := fmt.Sprintf("%s%s#%s", url, res.Location, keyString)
	logger.Println(loc)
}

func uploadReq(url string, req *scytale.UploadRequest) (*scytale.UploadResponse, error) {
	reqBuff := new(bytes.Buffer)
	err := json.NewEncoder(reqBuff).Encode(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", fmt.Sprintf("%s/ul", url), reqBuff)
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-Key", cfg.Key.String())

	client := new(http.Client)
	httpRes, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	} else if httpRes.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http status code: %d", httpRes.StatusCode)
	}
	defer httpRes.Body.Close()

	res := new(scytale.UploadResponse)
	err = json.NewDecoder(httpRes.Body).Decode(&res)
	if err != nil {
		return nil, err
	}

	if res.ErrorCode != scytale.ErrorCodeOK {
		return nil, fmt.Errorf("res error code: %d", res.ErrorCode)
	}

	return res, nil
}
