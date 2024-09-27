package main

import (
	"context"
	"flag"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

func main() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05",
	})

	var zoneID string
	var recordSet string
	flag.StringVar(&zoneID, "zone-id", "", "route53 zone id")
	flag.StringVar(&recordSet, "record-set", "", "route53 record set")
	flag.Parse()

	// get the ips
	var resp, err = http.Get("https://ipv4.icanhazip.com")
	if err != nil {
		log.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var ipv4 = net.ParseIP(strings.TrimSpace(string(body)))
	err = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Info("IPV4: ", ipv4)

	resp, err = http.Get("https://ipv6.icanhazip.com")
	if err != nil {
		log.Fatal(err)
	}
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var ipv6 = net.ParseIP(strings.TrimSpace(string(body)))
	err = resp.Body.Close()
	if err != nil {
		log.Fatal(err)
	}
	log.Info("IPV6: ", ipv6)

	var ctx = context.Background()
	awsConfig, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile("default"), config.WithRegion("us-west-2"))
	if err != nil {
		log.Fatalf("failed to load SDK configuration, %v", err)
	}
	var dnsClient = route53.NewFromConfig(awsConfig)

	var params = &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: &zoneID,
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{
				{
					Action: types.ChangeAction("UPSERT"),
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: &recordSet,
						Type: "A",
						TTL:  aws.Int64(300),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(ipv4.String()),
							},
						},
					},
				},
				{
					Action: types.ChangeAction("UPSERT"),
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: &recordSet,
						Type: "AAAA",
						TTL:  aws.Int64(300),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(ipv6.String()),
							},
						},
					},
				},
			},
		},
	}
	dnsResp, err := dnsClient.ChangeResourceRecordSets(ctx, params)
	if err != nil {
		log.Fatal(err)
	}
	var changeID = strings.ReplaceAll(*dnsResp.ChangeInfo.Id, "/change/", "")

	log.WithFields(log.Fields{
		"status":    dnsResp.ChangeInfo.Status,
		"id":        changeID,
		"submitted": dnsResp.ChangeInfo.SubmittedAt,
	}).Info("Submitted")

	// wait for propagation
	var status = dnsResp.ChangeInfo.Status
	for status == dnsResp.ChangeInfo.Status {
		time.Sleep(time.Second * 10)

		var statusParams = &route53.GetChangeInput{Id: &changeID}
		statusResp, err := dnsClient.GetChange(ctx, statusParams)
		if err != nil {
			log.Fatal(err)
		}
		status = statusResp.ChangeInfo.Status

		log.WithFields(log.Fields{
			"status":    statusResp.ChangeInfo.Status,
			"id":        changeID,
			"submitted": statusResp.ChangeInfo.SubmittedAt,
		}).Info("Waiting for propagation")
	}
}
