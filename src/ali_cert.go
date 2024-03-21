package main

import (
	"encoding/json"
	"errors"
	"fmt"
	cas20200407 "github.com/alibabacloud-go/cas-20200407/v2/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	util "github.com/alibabacloud-go/tea-utils/v2/service"
	"github.com/alibabacloud-go/tea/tea"
	"os"
	"strings"
	"time"
)

// CreateClient
// * 使用AK&SK初始化账号Client
// * @param accessKeyId
// * @param accessKeySecret
// * @return Client
// * @throws Exception
// */
func CreateClient(accessKeyId *string, accessKeySecret *string) (_result *cas20200407.Client, _err error) {
	config := &openapi.Config{
		// 必填，您的 AccessKey ID
		AccessKeyId: accessKeyId,
		// 必填，您的 AccessKey Secret
		AccessKeySecret: accessKeySecret,
	}
	// Endpoint 请参考 https://api.aliyun.com/product/cas
	config.Endpoint = tea.String("cas.aliyuncs.com")
	_result = &cas20200407.Client{}
	_result, _err = cas20200407.NewClient(config)
	return
}

func DescribeState(ct *cas20200407.Client, rt *util.RuntimeOptions) (err error) {
	request := &cas20200407.DescribePackageStateRequest{}
	defer func() {
		if r := tea.Recover(recover()); r != nil {
			err = r
		}
	}()
	respMessage, err := ct.DescribePackageStateWithOptions(request, rt)
	if err != nil {
		return
	}
	if *respMessage.Body.TotalCount > *respMessage.Body.UsedCount {
		return nil
	} else {
		return errors.New("no quota")
	}
}

func CreateCertRequest(ct *cas20200407.Client, rt *util.RuntimeOptions, dn *string) (rpb *cas20200407.CreateCertificateForPackageRequestResponseBody, err error) {
	request := &cas20200407.CreateCertificateForPackageRequestRequest{
		ValidateType: tea.String("FILE"),
		Domain:       tea.String(*dn),
	}
	defer func() {
		if r := tea.Recover(recover()); r != nil {
			err = r
		}
	}()
	respMessage, err := ct.CreateCertificateForPackageRequestWithOptions(request, rt)
	if err != nil {
		return nil, err
	}
	return respMessage.Body, nil
}

func OrderStats(ct *cas20200407.Client, rt *util.RuntimeOptions, oid *int64) (rpb *cas20200407.DescribeCertificateStateResponseBody, err error) {
	defer func() {
		if r := tea.Recover(recover()); r != nil {
			err = r
		}
	}()
	CertificateStateRequest := &cas20200407.DescribeCertificateStateRequest{
		OrderId: tea.Int64(*oid),
	}
	respMessage, err := ct.DescribeCertificateStateWithOptions(CertificateStateRequest, rt)
	if err != nil {
		return nil, err
	}
	return respMessage.Body, err
}

func CreateAuthFile(fl string, syspath string, tt string) (err error) {
	err = os.MkdirAll(syspath+".well-known/pki-validation", 0755)
	if err != nil {
		return
	}
	file, err := os.Create(syspath + fl)
	if err != nil {
		return
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			panic(err)
		}
	}(file)

	content := []byte(tt)
	_, err = file.Write(content)
	if err != nil {
		return
	}
	return nil
}

func writer(s string, n string) (err error) {
	//file, err := os.Create(n)
	file, err := os.OpenFile(n, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			return
		}
	}(file)

	content := []byte(s)
	_, err = file.Write(content)
	if err != nil {
		return
	}
	return nil
}

func _main(args []*string) (err error) {
	// 请确保代码运行环境设置了环境变量 ALIBABA_CLOUD_ACCESS_KEY_ID 和 ALIBABA_CLOUD_ACCESS_KEY_SECRET。
	// 工程代码泄露可能会导致 AccessKey 泄露，并威胁账号下所有资源的安全性。以下代码示例使用环境变量获取 AccessKey 的方式进行调用，仅供参考，建议使用更安全的 STS 方式，更多鉴权访问方式请参见：https://help.aliyun.com/document_detail/378661.html
	client, err := CreateClient(tea.String(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")), tea.String(os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")))
	if err != nil {
		return err
	}

	runtime := &util.RuntimeOptions{}
	err = DescribeState(client, runtime)
	if err != nil {
		return
	}

	respOrderId, err := CreateCertRequest(client, runtime, args[0])
	if err != nil {
		return
	}
	fmt.Println(*respOrderId.OrderId, *respOrderId.RequestId)
	//var id int64 = 10861963

	time.Sleep(time.Second * 5)

	certStats, err := OrderStats(client, runtime, respOrderId.OrderId)
	if err != nil {
		return
	}

	err = CreateAuthFile(*certStats.Uri, *args[1], *certStats.Content)
	if err != nil {
		return err
	}

	for i := 0; i < 10; i++ {
		time.Sleep(time.Second * 30)
		certStats, err := OrderStats(client, runtime, respOrderId.OrderId)
		if err != nil {
			continue
		}
		if *certStats.Type == "certificate" {
			err = writer(*certStats.PrivateKey, "key.pem")
			if err != nil {
				return
			}

			err = writer(*certStats.Certificate, "cert.pem")
			if err != nil {
				return
			}

			err = os.RemoveAll(*args[1] + ".well-known")
			if err != nil {
				return
			}
			break
		}
	}
	return nil
}

func main() {
	err := _main(tea.StringSlice(os.Args[1:]))
	if err != nil {
		var e = &tea.SDKError{}
		var _t *tea.SDKError
		if errors.As(err, &_t) {
			e = _t
		} else {
			e.Message = tea.String(err.Error())
		}
		// 错误 message
		fmt.Println(tea.StringValue(e.Message))
		// 诊断地址
		var data interface{}
		d := json.NewDecoder(strings.NewReader(tea.StringValue(e.Data)))
		err := d.Decode(&data)
		if err != nil {
			return
		}
		if m, ok := data.(map[string]interface{}); ok {
			recommend, _ := m["Recommend"]
			fmt.Println(recommend)
		}
		_, _err := util.AssertAsString(e.Message)
		if _err != nil {
			panic(_err)
		}
	}
	if err != nil {
		panic(err)
	}
}
