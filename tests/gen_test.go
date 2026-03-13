package tests

import (
	"os"
	"strings"
	"testing"
	"text/template"
)

// 生成玩家模块的初始代码
func TestGenPlayerComponentFile(t *testing.T) {
	genCfg := map[string]string{
		"ComponentName": "BaseInfo",
		"Comment":       "类的注释",
	}
	overrideOldFile := false // 如果有同名文件已经存在,是否覆盖

	// 读取模板文件
	tmplContent, err := os.ReadFile("./../template/player_component.template")
	if err != nil {
		t.Fatalf("读取模板文件失败: %v", err)
	}

	// 解析模板
	tmpl, err := template.New("component").Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("解析模板失败: %v", err)
	}

	// 生成输出文件
	fileName := "../game/" + genCfg["ComponentName"] + ".go"
	if !overrideOldFile {
		// 检查fileName是否已经存在
		if _, err := os.Stat(fileName); err == nil {
			t.Fatalf("文件 %s 已存在,防止覆盖,如需生成,请先删除旧文件或设置overrideOldFile为true", fileName)
		}
	}
	outFile, err := os.Create(fileName)
	if err != nil {
		t.Fatalf("创建输出文件失败: %v", err)
	}
	defer outFile.Close()

	// 执行模板
	err = tmpl.Execute(outFile, genCfg)
	if err != nil {
		t.Fatalf("执行模板失败: %v", err)
	}

	t.Logf("成功生成玩家组件文件: %v", fileName)
}

// 生成背包初始代码
func TestGenBagFile(t *testing.T) {
	// type {{.Name}}}}Bag struct {
	//	*{{.Container}}[*pb.{{.Message}}] `db:""`
	//}
	genCfg := map[string]string{
		"Comment":   "装备背包",
		"Name":      "Equip",
		"Message":   "Equip",
		"Container": "UniqueContainer",
	}
	overrideOldFile := false // 如果有同名文件已经存在,是否覆盖

	// 读取模板文件
	tmplContent, err := os.ReadFile("./../template/bag.template")
	if err != nil {
		t.Fatalf("读取模板文件失败: %v", err)
	}

	// 解析模板
	tmpl, err := template.New("component").Parse(string(tmplContent))
	if err != nil {
		t.Fatalf("解析模板失败: %v", err)
	}

	// 生成输出文件
	fileName := "../game/bag_" + strings.ToLower(genCfg["Name"]) + ".go"
	if !overrideOldFile {
		// 检查fileName是否已经存在
		if _, err := os.Stat(fileName); err == nil {
			t.Fatalf("文件 %s 已存在,防止覆盖,如需生成,请先删除旧文件或设置overrideOldFile为true", fileName)
		}
	}
	outFile, err := os.Create(fileName)
	if err != nil {
		t.Fatalf("创建输出文件失败: %v", err)
	}
	defer outFile.Close()

	// 执行模板
	err = tmpl.Execute(outFile, genCfg)
	if err != nil {
		t.Fatalf("执行模板失败: %v", err)
	}

	t.Logf("成功生成背包文件: %v", fileName)
}
