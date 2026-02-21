package main

import (
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("usage : download url filename")
		os.Exit(1)
	}
	url := os.Args[1]
	filename := os.Args[2]

	err := DownloadFile(url, filename)
	if err != nil {
		panic(err)
	}
}

const maxRetries = 10
const baseTime = 1 * time.Second
const maxBackoff = 32 * time.Second

func DownloadFile(url string, filepath string) error {
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	client := &http.Client{
		Timeout: 15 * baseTime,
	}
	var resp *http.Response
	//backoff logic writtne idk why but good practice
	for i := 0; i < maxRetries; i++ {
		resp, err = client.Get(url)
		//response handeling
		if err != nil {
			if i != maxRetries-1 {
				return err
			}
		} else if resp.StatusCode >= 500 || resp.StatusCode == http.StatusRequestTimeout {
			resp.Body.Close()
			if i == maxRetries-1 {
				return fmt.Errorf("server error : %d", resp.StatusCode)
			}
		} else {
			break
		}

		backoff := time.Duration(math.Min(
			float64(baseTime)*math.Pow(2, float64(i)),
			float64(maxBackoff),
		))
		jitter := time.Duration(rand.Float64() * float64(backoff) * 0.5)
		time.Sleep(backoff + jitter)
	}
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("request failed: %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	return nil
}
