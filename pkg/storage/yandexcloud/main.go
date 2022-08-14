package yandexcloud

import (
	"context"
	"errors"
	"regexp"

	dnsv1 "github.com/yandex-cloud/go-genproto/yandex/cloud/dns/v1"
	"github.com/yandex-cloud/go-sdk"
	"github.com/yandex-cloud/go-sdk/gen/dns"
	"github.com/yandex-cloud/go-sdk/iamkey"

	"github.com/sputnik-systems/prom-dns-http-sd/pkg/storage"
)

const ProviderName = "yandex-cloud/yandex"

type Client struct {
	folderIds []string
	dns       *dns.DNS
}

type Zone struct {
	client *Client
	zone   *dnsv1.DnsZone
}

type Record struct {
	record *dnsv1.RecordSet
}

func NewClient(ctx context.Context, filepath string, config *storage.Config) (storage.Client, error) {
	folderIds, err := getConfigFolderIds(config)
	if err != nil {
		return nil, err
	}

	key, err := iamkey.ReadFromJSONFile(filepath)
	if err != nil {
		return nil, err
	}

	creds, err := ycsdk.ServiceAccountKey(key)
	if err != nil {
		return nil, err
	}

	sdk, err := ycsdk.Build(ctx, ycsdk.Config{
		Credentials: creds,
	})

	return &Client{
		folderIds: folderIds,
		dns:       sdk.DNS(),
	}, nil
}

func (c *Client) ListZones(ctx context.Context, zones ...string) ([]storage.Zone, error) {
	z := []storage.Zone{}
	for _, folderId := range c.folderIds {
		it := c.dns.DnsZone().DnsZoneIterator(ctx, &dnsv1.ListDnsZonesRequest{
			FolderId: folderId,
		})

		zones, err := it.TakeAll()
		if err != nil {
			return nil, err
		}

		for _, zone := range zones {
			z = append(z, storage.Zone(&Zone{client: c, zone: zone}))
		}
	}

	return z, nil
}

func (z *Zone) ListRecords(ctx context.Context, filters ...string) ([]storage.Record, error) {
	re := []*regexp.Regexp{}
	for _, filter := range filters {
		rv, err := regexp.Compile(filter)
		if err != nil {
			return nil, err
		}

		re = append(re, rv)
	}

	it := z.client.dns.DnsZone().DnsZoneRecordSetsIterator(ctx, &dnsv1.ListDnsZoneRecordSetsRequest{
		DnsZoneId: z.zone.Id,
	})

	records, err := it.TakeAll()
	if err != nil {
		return nil, err
	}

	r := []storage.Record{}
	for _, record := range records {
		// var exists bool
		// for _, value := range r {
		// 	if value.GetName() == record.GetName() &&
		// 		value.GetType() == record.GetType() {
		// 		exists = true

		// 		break
		// 	}
		// }

		// if exists {
		// 	continue
		// }

		for _, rv := range re {
			if rv.MatchString(record.Name) {
				r = append(r, storage.Record(&Record{record: record}))

				break
			}
		}
	}

	return r, nil
}

func (r *Record) GetName() string {
	return r.record.GetName()
}

func (r *Record) GetType() string {
	return r.record.GetType()
}

func (r *Record) GetTTL() int64 {
	return r.record.GetTtl()
}

func (r *Record) GetData() []string {
	return r.record.GetData()
}

func getConfigFolderIds(c *storage.Config) ([]string, error) {
	fieldValue, ok := (c.Provider.Metadata["folderIds"]).([]interface{})
	if !ok {
		return nil, errors.New("incorrect provider definition, provider.metadata.folderIds filed required")
	}

	folderIds := []string{}
	for _, value := range fieldValue {
		folderId, ok := value.(string)
		if !ok {
			return nil, errors.New("incorrect provider.metadata.folderIds field type")
		}

		folderIds = append(folderIds, folderId)
	}

	return folderIds, nil
}
