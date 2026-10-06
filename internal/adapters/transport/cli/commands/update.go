package commands

import (
	"context"
	cliupdate "fkteams/internal/adapters/transport/cli/update"
	"fkteams/internal/app/config"

	ucli "github.com/urfave/cli/v3"
)

// updateCommand 创建 update 子命令
func updateCommand() *ucli.Command {
	return &ucli.Command{
		Name:  "update",
		Usage: "检查更新并升级到最新版本",
		Action: func(ctx context.Context, cmd *ucli.Command) error {
			if err := config.Init(); err != nil {
				return err
			}
			return cliupdate.SelfUpdate("beeteams", "axiao-feng", "beeassistant")
		},
	}
}
