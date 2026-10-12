package keychain

import (
	"testing"
	"time"
)

func TestParseKeychainDate(t *testing.T) {
	out := `keychain: "/Users/x/Library/Keychains/login.keychain-db"
attributes:
    "acct"<blob>="openhub.provider.bedrock.token.p1"
    "cdat"<timedate>=0x32303236313030363133353930375A00  "20261006135907Z\000"
    "mdat"<timedate>=0x32303236313030373038333031355A00  "20261007083015Z\000"
`
	got, err := parseKeychainDate(out, "mdat")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 7, 8, 30, 15, 0, time.UTC); !got.Equal(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if _, err := parseKeychainDate("nothing", "mdat"); err == nil {
		t.Fatal("expected an error")
	}
}
