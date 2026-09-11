package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/infrai-examples/nightly-edtech-snapshot/infrai"
	"github.com/infrai-examples/nightly-edtech-snapshot/snapshot"
)

type result struct {
	Bucket string `json:"bucket"`
	Key    string `json:"key"`
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}

func main() {
	log.SetFlags(0)
	source := flag.String("source", "", "immutable directory produced by the nightly export")
	bucket := flag.String("bucket", "", "destination storage bucket")
	dataset := flag.String("dataset", "learning-records", "path-safe dataset name")
	dateText := flag.String("date", time.Now().UTC().Format("2006-01-02"), "snapshot date in YYYY-MM-DD")
	flag.Parse()

	if *source == "" || *bucket == "" {
		log.Fatal("both -source and -bucket are required")
	}
	date, err := time.Parse("2006-01-02", *dateText)
	if err != nil {
		log.Fatalf("parse date: %v", err)
	}
	key, err := snapshot.ObjectKey(*dataset, date)
	if err != nil {
		log.Fatal(err)
	}
	archive, err := snapshot.ArchiveDirectory(*source)
	if err != nil {
		log.Fatal(err)
	}
	digestBytes := sha256.Sum256(archive)
	digest := hex.EncodeToString(digestBytes[:])

	client, err := infrai.NewClient(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := client.CreateBucket(ctx, *bucket); err != nil {
		log.Fatalf("create bucket: %v", err)
	}
	if err := client.PutObject(ctx, *bucket, key, archive, digest); err != nil {
		log.Fatalf("upload snapshot: %v", err)
	}

	encoded, err := json.Marshal(result{Bucket: *bucket, Key: key, Bytes: len(archive), SHA256: digest})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(string(encoded))
}
