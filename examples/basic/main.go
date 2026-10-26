// 文件用途：演示从公共模块导入手机号和金额组件，并输出转换结果。

package main

import (
	"fmt"
	"log"

	"github.com/anheicang/xinlin/base/phone"
	"github.com/anheicang/xinlin/base/points"
)

// main 演示公共金额和手机号组件的最小调用方式，输出转换后的示例值。
func main() {
	amount, err := points.Parse("12.34")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(amount, points.Format(amount))
	fmt.Println(phone.Normalize("+86 138-0013-8000"))
}
