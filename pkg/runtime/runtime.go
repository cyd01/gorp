package runtime

import (
	"log"

	"github.com/cyd01/gorp/pkg/config"
	"github.com/cyd01/gorp/pkg/server"
)

// Start is temporarily retained for compatibility during the migration to
// the server package.
//
// It will be removed at milestone A.4 once all callers have migrated to
// server.Build() followed by Server.Run().
func Start(cfg *config.Config) {
	srv, err := server.Build(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := srv.Run(); err != nil {
		log.Fatal(err)
	}
}
