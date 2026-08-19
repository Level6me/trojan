package cmd

import (
	"crypto/sha256"
	"fmt"
	"github.com/spf13/cobra"
	"trojan/core"
)

var passCmd = &cobra.Command{
	Use:   "pass [newPassword]",
	Short: "查看或重置Web管理员密码",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) > 0 {
			newPass := args[0]
			encryPass := sha256.Sum224([]byte(newPass))
			err := core.SetValue("admin_pass", fmt.Sprintf("%x", encryPass))
			if err != nil {
				fmt.Println("重置密码失败:", err)
			} else {
				fmt.Printf("成功将 admin 密码重置为: %s\n", newPass)
			}
		} else {
			pass, err := core.GetValue("admin_pass")
			if err != nil || pass == "" {
				fmt.Println("当前未设置 admin 密码")
			} else {
				fmt.Printf("当前 admin 密码哈希: %s\n", pass)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(passCmd)
}
