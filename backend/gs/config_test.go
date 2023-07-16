package gs

import (
	"testing"

	"github.com/konidev20/rapi/backend/test"
)

var configTests = []test.ConfigTestData[Config]{
	{S: "gs:bucketname:/", Cfg: Config{
		Bucket:      "bucketname",
		Prefix:      "",
		Connections: 5,
		Region:      "us",
	}},
	{S: "gs:bucketname:/prefix/directory", Cfg: Config{
		Bucket:      "bucketname",
		Prefix:      "prefix/directory",
		Connections: 5,
		Region:      "us",
	}},
	{S: "gs:bucketname:/prefix/directory/", Cfg: Config{
		Bucket:      "bucketname",
		Prefix:      "prefix/directory",
		Connections: 5,
		Region:      "us",
	}},
}

func TestParseConfig(t *testing.T) {
	test.ParseConfigTester(t, ParseConfig, configTests)
}
