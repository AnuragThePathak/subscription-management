package lib_test

import (
	"testing"

	"github.com/anuragthepathak/subscription-management/internal/lib"
	"github.com/stretchr/testify/assert"
)

func TestBuildMongoURI(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		host       string
		port       int
		username   string
		password   string
		dbName     string
		authSource string
		want       string
	}{
		{
			name:       "Standard Connection",
			host:       "localhost",
			port:       27017,
			username:   "admin",
			password:   "secret",
			dbName:     "sub_db",
			authSource: "admin",
			want:       "mongodb://admin:secret@localhost:27017/sub_db?authSource=admin",
		},
		{
			name:       "Atlas SRV Connection (Drops Port)",
			host:       "cluster0.abcde.mongodb.net",
			port:       27017, // This port should be ignored in the output!
			username:   "atlas_user",
			password:   "atlas_pass",
			dbName:     "sub_db",
			authSource: "admin",
			want:       "mongodb+srv://atlas_user:atlas_pass@cluster0.abcde.mongodb.net/sub_db?authSource=admin",
		},
		{
			name:       "URL Escaping for Special Characters",
			host:       "localhost.com",
			port:       27017,
			username:   "user@domain.com", // Contains '@'
			password:   "p@ssw:rd?#",      // Contains '@', ':', '?', '#'
			dbName:     "sub_db",
			authSource: "admin",
			// The '@' becomes '%40', ':' becomes '%3A', '?' becomes '%3F', '#' becomes '%23'
			want: "mongodb://user%40domain.com:p%40ssw%3Ard%3F%23@localhost.com:27017/sub_db?authSource=admin",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := lib.BuildMongoURI(tt.host, tt.port, tt.username, tt.password, tt.dbName, tt.authSource)
			assert.Equal(t, tt.want, got)
		})
	}
}
