package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"bit-labs.cn/owl/utils"
	"github.com/spf13/cobra"
)

// NewUtilsCertCmd 返回顶层命令 utils:cert，子命令为 gen-key / gen-license / verify-license。
func NewUtilsCertCmd() *cobra.Command {
	certCmd := &cobra.Command{
		Use:   "utils:cert",
		Short: "证书与授权文件工具",
		Long:  "一键直达：根命令下执行 `utils:cert gen-key` 等，无需多层前缀。",
	}

	certCmd.AddCommand(
		newGenKeyCmd(),
		newGenLicenseCmd(),
		newVerifyLicenseCmd(),
	)
	return certCmd
}

func newGenKeyCmd() *cobra.Command {
	var bits int
	var privateOut string
	var publicOut string

	cmd := &cobra.Command{
		Use:   "gen-key",
		Short: "生成 RSA 公私钥",
		RunE: func(cmd *cobra.Command, args []string) error {
			privateKey, publicKey, err := utils.GenRSAKeyPair(bits)
			if err != nil {
				return err
			}

			if err = os.WriteFile(privateOut, []byte(privateKey), 0644); err != nil {
				return err
			}
			if err = os.WriteFile(publicOut, []byte(publicKey), 0644); err != nil {
				return err
			}

			fmt.Printf("私钥已写入: %s\n", privateOut)
			fmt.Printf("公钥已写入: %s\n", publicOut)
			return nil
		},
	}

	cmd.Flags().IntVar(&bits, "bits", 2048, "RSA 密钥位数")
	cmd.Flags().StringVar(&privateOut, "private-out", "private.pem", "私钥输出文件")
	cmd.Flags().StringVar(&publicOut, "public-out", "public.pem", "公钥输出文件")
	return cmd
}

func newGenLicenseCmd() *cobra.Command {
	var privateKeyFile string
	var payloadFile string
	var outFile string

	cmd := &cobra.Command{
		Use:   "gen-license",
		Short: "根据 payload.json 生成签名授权文件",
		RunE: func(cmd *cobra.Command, args []string) error {
			privateKeyBytes, err := os.ReadFile(privateKeyFile)
			if err != nil {
				return err
			}

			payloadBytes, err := os.ReadFile(payloadFile)
			if err != nil {
				return err
			}

			var payload utils.LicensePayload
			if err = json.Unmarshal(payloadBytes, &payload); err != nil {
				return err
			}

			licenseBytes, err := utils.GenLicense(string(privateKeyBytes), payload)
			if err != nil {
				return err
			}

			if err = os.WriteFile(outFile, licenseBytes, 0644); err != nil {
				return err
			}

			fmt.Printf("授权文件已写入: %s\n", outFile)
			return nil
		},
	}

	cmd.Flags().StringVar(&privateKeyFile, "private-key-file", "private.pem", "私钥 PEM 文件")
	cmd.Flags().StringVar(&payloadFile, "payload-file", "payload.json", "授权负载 JSON 文件")
	cmd.Flags().StringVar(&outFile, "out", "license.lic", "授权文件输出路径")
	return cmd
}

func newVerifyLicenseCmd() *cobra.Command {
	var publicKeyFile string
	var licenseFile string

	cmd := &cobra.Command{
		Use:   "verify-license",
		Short: "使用公钥验证授权文件",
		RunE: func(cmd *cobra.Command, args []string) error {
			publicKeyBytes, err := os.ReadFile(publicKeyFile)
			if err != nil {
				return err
			}

			ok, payload := utils.VerifyLicense(string(publicKeyBytes), licenseFile)
			if !ok {
				return fmt.Errorf("授权验证失败")
			}

			b, _ := json.MarshalIndent(payload, "", "  ")
			fmt.Printf("授权负载:\n%s\n", string(b))
			return nil
		},
	}

	cmd.Flags().StringVar(&publicKeyFile, "public-key-file", "public.pem", "公钥 PEM 文件")
	cmd.Flags().StringVar(&licenseFile, "license-file", "license.lic", "授权文件路径")
	return cmd
}
