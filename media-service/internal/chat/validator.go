// Package chat 存放和"聊天消息"相关的业务逻辑。
// 目前只有一个校验器，负责在消息广播出去之前做基础的合法性检查。
package chat

import (
	"errors"         // Go 标准库：用来创建错误值（error）
	"strings"        // Go 标准库：字符串处理工具（去空格、拼接等）
	"unicode/utf8"   // Go 标准库：处理 UTF-8 编码，例如按"字符"数（rune）来计数而不是按字节数
)

// 【小知识】var (...) 是 Go 里的分组变量声明写法，等价于分别写多个 var。
// errors.New(...) 会创建一个只有字符串消息的错误值。
// 我们把这两个错误做成"包级别变量"，调用方就可以用 errors.Is(err, chat.ErrEmptyContent)
// 来判断具体是哪种错误，比"字符串比较"更安全。
var (
	// ErrEmptyContent 表示聊天消息内容是空的（去掉首尾空白之后什么都没剩）。
	ErrEmptyContent = errors.New("chat content is empty")
	// ErrContentTooLong 表示聊天消息过长（超过 1000 个字符，注意是"字符"不是"字节"）。
	ErrContentTooLong = errors.New("chat content exceeds 1000 characters")
)

// ValidateContent 校验一条聊天消息的内容是否合法。
//
// 参数：
//   content —— 用户从前端发来的原始文本
//
// 返回值：
//   string —— 清洗后（去除首尾空白）的合法内容，可以直接拿去广播
//   error  —— 若为 nil 表示校验通过；否则是上面定义的两种错误之一
//
// 【为什么要有这个函数】
// 服务端永远不能相信客户端。前端可能因为 bug 或恶意用户发来一条空消息或一条超长消息，
// 如果直接广播，会把无意义或超大消息推给房间里的其他人。所以我们在广播前统一走这里做一次卡口。
func ValidateContent(content string) (string, error) {
	// strings.TrimSpace 会去掉字符串首尾的空格、\n、\t 等所有空白字符。
	// 这样"   \n"这种"看起来有东西"但实际上没内容的消息会被判定为空。
	trimmed := strings.TrimSpace(content)

	// 如果去空白后是空字符串，直接返回 ErrEmptyContent 错误。
	// 【Go 惯用法】出错时返回"零值 + error"：这里第一个返回值给 ""（string 的零值）。
	if trimmed == "" {
		return "", ErrEmptyContent
	}

	// 【重点】len(s) 得到的是"字节数"，而中文一个字符通常占 3 个字节（UTF-8 编码下）。
	// 如果用 len 来判断"1000 字符"，一条 400 个汉字的消息就会被误判成超长。
	// utf8.RuneCountInString 按 Unicode 码点（rune）计数，1 个中文字 = 1 个 rune，才是我们想要的。
	if utf8.RuneCountInString(trimmed) > 1000 {
		return "", ErrContentTooLong
	}

	// 校验通过，返回清洗后的内容和 nil（表示没有错误）。
	return trimmed, nil
}
