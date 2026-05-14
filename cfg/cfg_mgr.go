package cfg

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/fish-tennis/gserver/internal"
	"google.golang.org/protobuf/encoding/protodelim"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

var (
	// 配置数据文件扩展名,默认为".json",可设置为".pb"以加载protobuf格式
	// 在调用Load之前设置此变量来控制加载格式
	DataFileExt = ".json"
)

// map类型的配置数据管理
type DataMap[E internal.CfgData] struct {
	Elems map[int32]E
}

func NewDataMap[E internal.CfgData]() *DataMap[E] {
	return &DataMap[E]{
		Elems: make(map[int32]E),
	}
}

func (this *DataMap[E]) GetCfg(cfgId int32) E {
	return this.Elems[cfgId]
}

func (this *DataMap[E]) Range(f func(e E) bool) {
	for _, cfg := range this.Elems {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据
func (this *DataMap[E]) Load(fileName string) error {
	if this.Elems == nil {
		this.Elems = make(map[int32]E)
	}
	if strings.HasSuffix(fileName, ".json") {
		return this.LoadJson(fileName)
	}
	if strings.HasSuffix(fileName, ".pb") {
		return this.LoadPb(fileName)
	}
	return errors.New("unsupported file type")
}

// 从json文件加载数据
func (this *DataMap[E]) LoadJson(fileName string) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	cfgMap := make(map[int32]E)
	err = json.Unmarshal(fileData, &cfgMap)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	this.Elems = cfgMap
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.Elems))
	return nil
}

// 创建索引
func (this *DataMap[E]) CreateIndexInt32(indexFn func(e E) int32) map[int32]*DataMap[E] {
	indexMap := make(map[int32]*DataMap[E])
	for _, e := range this.Elems {
		index := indexFn(e)
		if indexMap[index] == nil {
			indexMap[index] = NewDataMap[E]()
		}
		indexMap[index].Elems[e.GetCfgId()] = e
	}
	return indexMap
}

// 创建子集
func (this *DataMap[E]) CreateSubset(filter func(e E) bool) *DataMap[E] {
	subMap := NewDataMap[E]()
	for _, e := range this.Elems {
		if filter(e) {
			subMap.Elems[e.GetCfgId()] = e
		}
	}
	return subMap
}

// 创建slice
func (this *DataMap[E]) CreateSlice(filter func(e E) bool, cmpFn func(a, b E) int) []E {
	var s []E
	for _, e := range this.Elems {
		if filter(e) {
			s = append(s, e)
		}
	}
	if cmpFn != nil {
		slices.SortFunc(s, cmpFn)
	}
	return s
}

// 从pb文件加载数据
func (this *DataMap[E]) LoadPb(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		slog.Error("LoadPbErr", "fileName", fileName, "err", err)
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	cfgMap := make(map[int32]E)
	for {
		cfg, newErr := newElement[E]()
		if newErr != nil {
			slog.Error("LoadPbErr", "fileName", fileName, "err", newErr)
			return newErr
		}
		msg, ok := any(cfg).(proto.Message)
		if !ok {
			return fmt.Errorf("type %T does not implement proto.Message", cfg)
		}
		err = protodelim.UnmarshalFrom(reader, msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("LoadPbErr", "fileName", fileName, "err", err)
			return err
		}
		cfgMap[cfg.GetCfgId()] = cfg
	}
	this.Elems = cfgMap
	slog.Info("LoadPb", "fileName", fileName, "count", len(this.Elems))
	return nil
}

// map[string]E类型的配置数据管理
type StrDataMap[E internal.StrCfgData] struct {
	Elems map[string]E
}

func NewStrDataMap[E internal.StrCfgData]() *StrDataMap[E] {
	return &StrDataMap[E]{
		Elems: make(map[string]E),
	}
}

func (m *StrDataMap[E]) GetCfg(cfgId string) E {
	return m.Elems[cfgId]
}

func (m *StrDataMap[E]) Range(f func(e E) bool) {
	for _, cfg := range m.Elems {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据,支持json和csv
func (m *StrDataMap[E]) Load(fileName string) error {
	if m.Elems == nil {
		m.Elems = make(map[string]E)
	}
	return m.LoadJson(fileName)
}

// 从json文件加载数据
func (m *StrDataMap[E]) LoadJson(fileName string) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	cfgMap := make(map[string]E)
	err = json.Unmarshal(fileData, &cfgMap)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	m.Elems = cfgMap
	slog.Info("LoadJson", "fileName", fileName, "count", len(m.Elems))
	return nil
}

// 创建子集
func (m *StrDataMap[E]) CreateSubset(filter func(e E) bool) *StrDataMap[E] {
	subMap := NewStrDataMap[E]()
	for _, e := range m.Elems {
		if filter(e) {
			subMap.Elems[e.GetCfgId()] = e
		}
	}
	return subMap
}

// 创建slice
func (m *StrDataMap[E]) CreateSlice(filter func(e E) bool, cmpFn func(a, b E) int) []E {
	var s []E
	for _, e := range m.Elems {
		if filter(e) {
			s = append(s, e)
		}
	}
	if cmpFn != nil {
		slices.SortFunc(s, cmpFn)
	}
	return s
}

// slice类型的配置数据管理
type DataSlice[E any] struct {
	Elems []E
}

func (this *DataSlice[E]) Len() int {
	return len(this.Elems)
}

func (this *DataSlice[E]) GetCfg(index int) E {
	return this.Elems[index]
}

func (this *DataSlice[E]) Range(f func(e E) bool) {
	for _, cfg := range this.Elems {
		if !f(cfg) {
			return
		}
	}
}

// 加载配置数据
func (this *DataSlice[E]) Load(fileName string) error {
	if strings.HasSuffix(fileName, ".json") {
		return this.LoadJson(fileName)
	}
	if strings.HasSuffix(fileName, ".pb") {
		return this.LoadPb(fileName)
	}
	return errors.New("unsupported file type")
}

// 从json文件加载数据
func (this *DataSlice[E]) LoadJson(fileName string) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	var cfgList []E
	err = json.Unmarshal(fileData, &cfgList)
	if err != nil {
		slog.Error("LoadJsonErr", "fileName", fileName, "err", err)
		return err
	}
	this.Elems = cfgList
	slog.Info("LoadJson", "fileName", fileName, "count", len(this.Elems))
	this.checkDuplicateCfgId(fileName)
	return nil
}

// 从pb文件加载数据
func (this *DataSlice[E]) LoadPb(fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		slog.Error("LoadPbErr", "fileName", fileName, "err", err)
		return err
	}
	defer file.Close()
	reader := bufio.NewReader(file)
	var cfgList []E
	for {
		cfg, newErr := newElement[E]()
		if newErr != nil {
			slog.Error("LoadPbErr", "fileName", fileName, "err", newErr)
			return newErr
		}
		msg, ok := any(cfg).(proto.Message)
		if !ok {
			return fmt.Errorf("type %T does not implement proto.Message", cfg)
		}
		err = protodelim.UnmarshalFrom(reader, msg)
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("LoadPbErr", "fileName", fileName, "err", err)
			return err
		}
		cfgList = append(cfgList, cfg)
	}
	this.Elems = cfgList
	slog.Info("LoadPb", "fileName", fileName, "count", len(this.Elems))
	this.checkDuplicateCfgId(fileName)
	return nil
}

// 如果配置项是CfgData,检查id是否重复
func (this *DataSlice[E]) checkDuplicateCfgId(fileName string) {
	for i := 0; i < len(this.Elems); i++ {
		cfgDataI, ok := any(this.Elems[i]).(internal.CfgData)
		if !ok {
			return
		}
		for j := i + 1; j < len(this.Elems); j++ {
			cfgDataJ := any(this.Elems[j]).(internal.CfgData)
			if cfgDataI.GetCfgId() == cfgDataJ.GetCfgId() {
				slog.Error("duplicate id", "fileName", fileName, "id", cfgDataI.GetCfgId())
			}
		}
	}
}

func LoadObjectFromJson(fileName string, obj proto.Message) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadObjectFromFileErr", "fileName", fileName, "err", err)
		return err
	}
	err = protojson.Unmarshal(fileData, obj)
	if err != nil {
		slog.Error("LoadObjectFromFileErr", "fileName", fileName, "err", err)
		return err
	}
	return nil
}

func LoadObjectFromPb(fileName string, obj proto.Message) error {
	fileData, err := os.ReadFile(fileName)
	if err != nil {
		slog.Error("LoadObjectFromFileErr", "fileName", fileName, "err", err)
		return err
	}
	err = proto.Unmarshal(fileData, obj)
	if err != nil {
		slog.Error("LoadObjectFromFileErr", "fileName", fileName, "err", err)
		return err
	}
	return nil
}

func ResolveDataFile(fileName string) string {
	return EnsureDataFileExt(fileName, DataFileExt)
}

func EnsureDataFileExt(fileName, ext string) string {
	if len(ext) > 0 && ext[0] != '.' {
		ext = "." + ext
	}
	if strings.HasSuffix(fileName, ext) {
		return fileName
	}
	return strings.TrimSuffix(fileName, filepath.Ext(fileName)) + ext
}

func newElement[E any]() (E, error) {
	var zero E
	t := reflect.TypeOf(zero)
	if t == nil {
		return zero, errors.New("invalid nil element type")
	}
	if t.Kind() != reflect.Ptr {
		return zero, fmt.Errorf("element type must be pointer, got %v", t)
	}
	v := reflect.New(t.Elem()).Interface()
	elem, ok := v.(E)
	if !ok {
		return zero, fmt.Errorf("failed to cast element to generic type %T", zero)
	}
	return elem, nil
}

type loadable interface {
	Load(filename string) error
}

func LoadConfig[L loadable](filter func(string) bool, fileName, dataDir string, newFn func() L, target *L) error {
	if filter != nil && !filter(fileName) {
		return nil
	}
	resolvedFileName := ResolveDataFile(dataDir + fileName)
	tmp := newFn()
	if err := tmp.Load(resolvedFileName); err != nil {
		return err
	}
	*target = tmp
	return nil
}

func LoadObjectConfig[T proto.Message](filter func(string) bool, fileName, dataDir string, newFn func() T, target *T) error {
	if filter != nil && !filter(fileName) {
		return nil
	}
	resolvedFileName := ResolveDataFile(dataDir + fileName)
	tmp := newFn()
	var err error
	if strings.HasSuffix(resolvedFileName, ".pb") {
		err = LoadObjectFromPb(resolvedFileName, tmp)
	} else {
		err = LoadObjectFromJson(resolvedFileName, tmp)
	}
	if err != nil {
		return err
	}
	*target = tmp
	return nil
}

func Process[T any](fn func(T) error, data T) error {
	if fn != nil {
		return fn(data)
	}
	return nil
}
