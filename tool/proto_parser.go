package tool

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// proto的message的结构信息
type ProtoMessageStructInfo struct {
	protoName   string
	messageName string
	f           *ast.File
	genDecl     *ast.GenDecl
	typeSpec    *ast.TypeSpec
	structType  *ast.StructType
	keyComment  string
}

// 代码模板
type CodeTemplate struct {
	// 注释关键字,如@Player
	keyComment  string

	// 生成文件名
	outFile string

	/*
	package game
	import "github.com/fish-tennis/gserver/pb"
	*/
	// 文件头
	header string

	/*
	@Player对应的函数模板:
	func (this *Player) Send{MessageName}(packet *pb.{MessageName}) bool {
		return this.Send(Cmd(pb.Cmd{ProtoFileName}_Cmd_{MessageName}), packet)
	}
	@Server对应的函数模板
	func SendPacket{MessageName}(conn Connection, packet *pb.{MessageName}) bool {
		return conn.Send(Cmd(pb.Cmd{ProtoFileName}_Cmd_{MessageName}), packet)
	}
	*/
	// 函数替换模板
	funcTemplate string
}

type ParserResult struct {
	keys []string
	codeTemplates []*CodeTemplate
	structInfos []*ProtoMessageStructInfo
}

func (this *ParserResult) GetCodeTemplate(key string) *CodeTemplate {
	for _,v := range this.codeTemplates {
		if v.keyComment == key {
			return v
		}
	}
	return nil
}

// 解析protoc-gen-go生成的*pb.go代码
// 参考github.com/favadi/protoc-go-inject-tag
// protoc-go-inject-tag只能对Message的字段(field)进行处理,不能完全满足我们的需求,我们希望直接对Message(struct)的注释进行解析
// 而且golang的反射只能获取field的tag,struct自身没有tag,因此如果我们希望对struct进行特殊标记,生成的*pb.go代码,也无法处理struct
// 所以我们的解决方案是,直接利用golang的parser接口,解析出struct的注释信息,根据在注释里插入的关键字,生成辅助代码,应用层调用生成的辅助代码时,
// 由于不需要再进行反射操作,性能也没有损失
func ParseProtoCode(protoCodeFile string, parserResult *ParserResult) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, protoCodeFile, nil, parser.ParseComments)
	if err != nil {
		return
	}

	for _, decl := range f.Decls {
		// check if is generic declaration
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		var typeSpec *ast.TypeSpec
		for _, spec := range genDecl.Specs {
			if ts, tsOK := spec.(*ast.TypeSpec); tsOK {
				typeSpec = ts
				break
			}
		}

		// skip if can't get type spec
		if typeSpec == nil {
			continue
		}
		//println(fmt.Sprintf("typeSpec:%v", typeSpec))

		// not a struct, skip
		structDecl, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}
		//println(fmt.Sprintf("struct doc:%v", genDecl.Doc))
		// struct的注释在genDecl.Doc里
		if genDecl.Doc != nil {
			keyChecker := make(map[string]struct{})
			for _,structComment := range genDecl.Doc.List {
				comment := strings.TrimPrefix(structComment.Text, "//")
				comment = strings.TrimSpace(comment)
				if parserResult.GetCodeTemplate(comment) != nil {
					// 排重
					if _,ok := keyChecker[comment]; ok {
						continue
					}
					structInfo := &ProtoMessageStructInfo{
						protoName:   path.Base(path.Clean(strings.Replace(protoCodeFile,"\\","/",-1))),
						messageName: typeSpec.Name.Name,
						f:           f,
						genDecl:     genDecl,
						typeSpec:    typeSpec,
						structType:  structDecl,
						keyComment:  comment,
					}
					parserResult.structInfos = append(parserResult.structInfos, structInfo)
					keyChecker[comment] = struct{}{}
					println(fmt.Sprintf("%v keyComment:%v", structInfo.messageName, comment))
					break
				}
			}
		}
		//println(fmt.Sprintf("structDecl:%v", structDecl))
		//
		//for _, field := range structDecl.Fields.List {
		//	println(fmt.Sprintf("field:%v", field))
		//}
	}
}

func ParseFiles(pattern string) {
	parserResult := &ParserResult{
		structInfos: make([]*ProtoMessageStructInfo,0),
	}
	// @Player模板
	parserResult.codeTemplates = append(parserResult.codeTemplates, createPlayerCodeTemplate())
	// @Server模板
	parserResult.codeTemplates = append(parserResult.codeTemplates, createServerCodeTemplate())
	files, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatal(err)
	}
	for _, path := range files {
		finfo, err := os.Stat(path)
		if err != nil {
			log.Fatal(err)
		}

		if finfo.IsDir() {
			continue
		}

		// It should end with ".pb.go" at a minimum.
		if !strings.HasSuffix(strings.ToLower(finfo.Name()), ".pb.go") {
			continue
		}

		ParseProtoCode(path, parserResult)
	}
	generateCode(parserResult, "@Player")
	generateCode(parserResult, "@Server")
}

// 生成相应的辅助代码
func generateCode(parserResult *ParserResult, key string) {
	codeTemplate := parserResult.GetCodeTemplate(key)
	builder := strings.Builder{}
	builder.WriteString(codeTemplate.header)
	for _,structInfo := range parserResult.structInfos {
		if structInfo.keyComment != codeTemplate.keyComment {
			continue
		}
		protoFileName := structInfo.protoName[:strings.Index(structInfo.protoName,".pb.go")]
		// 首字母大写
		protoFileName = strings.ToUpper(protoFileName[:1]) + protoFileName[1:]
		messageName := structInfo.messageName
		//cmdName := fmt.Sprintf("Cmd%v_Cmd_%v", protoFileName, messageName)
		funcStr := codeTemplate.funcTemplate
		funcStr = strings.ReplaceAll(funcStr, "{MessageName}", messageName)
		funcStr = strings.ReplaceAll(funcStr, "{ProtoFileName}", protoFileName)
		builder.WriteString(funcStr)
		//builder.WriteString("\n")
	}
	println(builder.String())
	os.WriteFile(codeTemplate.outFile, ([]byte)(builder.String()), 0644)
}

// 玩家消息模板
func createPlayerCodeTemplate() *CodeTemplate {
	return &CodeTemplate{
		keyComment: "@Player",
		outFile: "./../game/player_send_gen.go",
		header: `// Code generated by proto_parser. DO NOT EDIT.
// 对应的proto规范:
// 如玩家有个Money的组件,该组件相关的proto文件名:money.proto
//  enum CmdMoney {
//    Cmd_CoinRes = 1102; // 格式: Cmd_MessageName
//  }
//
//  // @Player
//  message CoinRes {
//    int32 totalCoin = 1; // 当前总值
//  }
package game

import "github.com/fish-tennis/gserver/pb"
`,
		funcTemplate: `
func (this *Player) Send{MessageName}(packet *pb.{MessageName}) bool {
	return this.Send(Cmd(pb.Cmd{ProtoFileName}_Cmd_{MessageName}), packet)
}
`,
	}
}

// 普通服务器消息模板
func createServerCodeTemplate() *CodeTemplate {
	return &CodeTemplate{
		keyComment: "@Server",
		outFile:    "./../game/server_send_gen.go",
		header: `// Code generated by proto_parser. DO NOT EDIT.
// 对应的proto规范:
//  xxx.proto
//  enum CmdXXX {
//    Cmd_YYY = 1102; // 格式: Cmd_MessageName
//  }
//
//  // @Server
//  message YYY {
//    int32 abc = 1;
//  }
package game

import "github.com/fish-tennis/gserver/pb"
`,
		funcTemplate: `
func SendPacket{MessageName}(conn Connection, packet *pb.{MessageName}) bool {
	return conn.Send(Cmd(pb.Cmd{ProtoFileName}_Cmd_{MessageName}), packet)
}
`,
	}
}