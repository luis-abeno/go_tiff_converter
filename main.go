package main

import (
    "bytes"
    "fmt"
    "image/png"
    "io/ioutil"
    "log"
    "os"
    "path/filepath"
    "sync"
    "time"

    tiff "github.com/chai2010/tiff"
)

type ImageResult struct {
    Index int
    Data  []byte
    Err   error
}

func main() {
    startTime := time.Now()
    log.Printf("Process started at: %s\n", startTime.Format(time.RFC3339))

    imageName := "IMG_00000073_00073530.tiff"

    b, err := ioutil.ReadFile(imageName)
    if err != nil {
        panic(err)
    }

    // Create "converted" folder if it doesn't exist
    if _, err := os.Stat("converted"); os.IsNotExist(err) {
        err = os.Mkdir("converted", 0755)
        if err != nil {
            log.Fatal(err)
        }
    }

    // Decode tiff
    m, errors, err := tiff.DecodeAll(bytes.NewReader(b))
    if err != nil {
        log.Println(err)
    }

    var wg sync.WaitGroup
    sem := make(chan struct{}, 10) // Limit to 10 concurrent goroutines
    results := make(chan ImageResult, len(m)*len(m[0]))

    // Encode to PNG and save in parallel
    for i := 0; i < len(m); i++ {
        for j := 0; j < len(m[i]); j++ {
            wg.Add(1)
            sem <- struct{}{} // Acquire a slot
            go func(i, j int) {
                defer wg.Done()
                defer func() { <-sem }() // Release the slot

                fmt.Sprintf("converted/%s-%02d-%02d.png", filepath.Base(imageName), i, j)
                if errors[i][j] != nil {
                    results <- ImageResult{Index: i*len(m[i]) + j, Err: errors[i][j]}
                    return
                }

                var buf bytes.Buffer
                if err := png.Encode(&buf, m[i][j]); err != nil {
                    results <- ImageResult{Index: i*len(m[i]) + j, Err: err}
                    return
                }
                results <- ImageResult{Index: i*len(m[i]) + j, Data: buf.Bytes()}
            }(i, j)
        }
    }

    go func() {
        wg.Wait()
        close(results)
    }()

    // Collect results and write to disk in order
    for result := range results {
        if result.Err != nil {
            log.Printf("Failed to process image %d: %v\n", result.Index, result.Err)
            continue
        }
        newname := fmt.Sprintf("converted/%s-%02d.png", filepath.Base(imageName), result.Index)
        if err := ioutil.WriteFile(newname, result.Data, 0666); err != nil {
            log.Printf("Failed to write %s: %v\n", newname, err)
            continue
        }
        fmt.Printf("Save %s ok\n", newname)
    }

    endTime := time.Now()
    log.Printf("Process ended at: %s\n", endTime.Format(time.RFC3339))
    log.Printf("Total duration: %s\n", endTime.Sub(startTime))
}