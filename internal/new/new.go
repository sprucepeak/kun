package new

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/sprucepeak/kun/config"
	"github.com/sprucepeak/kun/pkg/helper"
	"github.com/sprucepeak/kun/pkg/output"
	"github.com/sprucepeak/kun/tpl"
)

type Project struct {
	ProjectName string `survey:"name"`
}

var CmdNew = &cobra.Command{
	Use:     "new",
	Example: "kun new demo",
	Short:   "create a new project.",
	Long:    `create a new project with kun layout.`,
	RunE:    run,
}

func init() {
	// repo-url flag 只在 RunE 内通过 cmd.Flags().GetString 读取，
	// 避免包级变量在多次调用间泄漏（对比 gen.go 的正确做法）。
	CmdNew.Flags().StringP("repo-url", "g", "", "layout repo")
}

// Register 将 new 子命令挂载到 parent。
func Register(parent *cobra.Command) {
	parent.AddCommand(CmdNew)
}

func NewProject() *Project {
	return &Project{}
}

func run(cmd *cobra.Command, args []string) error {
	// B4 fix: 从 flag 读取而非包级变量
	repoURL, _ := cmd.Flags().GetString("repo-url")

	p := NewProject()
	switch len(args) {
	case 0:
		err := survey.AskOne(&survey.Input{
			Message: "What is your project name?",
			Help:    "project name.",
			Suggest: nil,
		}, &p.ProjectName, survey.WithValidator(survey.Required))
		if err != nil {
			// 交互中断(Ctrl+C / EOF)不计为错误,静默退出
			return nil
		}
	case 1:
		p.ProjectName = args[0]
	default:
		return fmt.Errorf("accepts %d arg(s), received %d", 1, len(args))
	}

	p.ProjectName = strings.TrimSpace(p.ProjectName)
	if p.ProjectName == "." || p.ProjectName == ".." || filepath.IsAbs(p.ProjectName) ||
		strings.ContainsAny(p.ProjectName, `/\`) {
		return fmt.Errorf("invalid project name %q: must be a relative directory name without path separators", p.ProjectName)
	}

	// clone repo
	yes, err := p.cloneTemplate(repoURL)
	if err != nil || !yes {
		return err
	}

	if err = p.replacePackageName(); err != nil {
		return err
	}
	if err = p.modTidy(); err != nil {
		return err
	}
	p.rmGit()
	if err := p.installWire(); err != nil {
		return fmt.Errorf("project created but wire install failed: %w", err)
	}
	output.Success("Project [ %s ] created successfully!", p.ProjectName)
	output.Success("Done. Now run:")
	output.Success("› cd %s ", p.ProjectName)
	output.Success("› kun run \n")
	return nil
}

func (p *Project) cloneTemplate(repoURL string) (bool, error) {

	stat, _ := os.Stat(p.ProjectName)
	if stat != nil {
		var overwrite = false

		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Folder %s already exists, do you want to overwrite it?", p.ProjectName),
			Help:    "Remove old project and create new project.",
		}
		err := survey.AskOne(prompt, &overwrite)
		if err != nil {
			// 交互中断,静默退出
			return false, nil
		}
		if !overwrite {
			return false, nil
		}
		if err = os.RemoveAll(p.ProjectName); err != nil {
			return false, fmt.Errorf("remove old project error: %w", err)
		}
	}

	if repoURL == "" {
		layout := ""
		prompt := &survey.Select{
			Message: "Please select a layout:",
			Options: []string{
				"Advanced",
				"Basic",
			},
			Description: func(value string, index int) string {
				if index == 1 {
					return "A basic project structure"
				}
				return "It has rich functions such as db, jwt, cron, migration, test, etc"
			},
		}
		err := survey.AskOne(prompt, &layout)
		if err != nil {
			// 交互中断,静默退出
			return false, nil
		}

		templateName := "basic"
		if layout != "Basic" {
			templateName = "advanced"
		}

		output.Success("Generate code from template: %s", templateName)

		if err = handlerZip(p.ProjectName, templateName); err != nil {
			return false, fmt.Errorf("generate code from template %s error: %w", templateName, err)
		}

	} else { // clone from repoURL
		output.Success("git clone %s", repoURL)
		cmd := exec.Command("git", "clone", repoURL, p.ProjectName)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return false, fmt.Errorf("git clone %s error: %w\n%s", repoURL, err, string(out))
		}
	}
	return true, nil
}

func (p *Project) replacePackageName() error {
	packageName, err := helper.GetProjectName(p.ProjectName)
	if err != nil {
		return err
	}

	if err = p.replaceFiles(packageName); err != nil {
		return err
	}

	cmd := exec.Command("go", "mod", "edit", "-module", p.ProjectName)
	cmd.Dir = p.ProjectName
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("go mod edit error: %w\n%s", err, string(out))
	}
	return nil
}

func (p *Project) modTidy() error {
	output.Success("go mod tidy")
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = p.ProjectName
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy error: %w", err)
	}
	return nil
}

func (p *Project) rmGit() {
	_ = os.RemoveAll(p.ProjectName + "/.git")
}

func (p *Project) installWire() error {
	if _, err := exec.LookPath("wire"); err == nil {
		output.Success("wire is already installed, skipping go install.")
		return nil
	}
	output.Success("go install %s", config.WireUrl)
	cmd := exec.Command("go", "install", config.WireUrl)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go install %s: %w", config.WireUrl, err)
	}
	return nil
}

func (p *Project) replaceFiles(packageName string) error {
	// 只替换 import path 前缀("packageName/"),避免全文替换误伤注释/字符串里的同名词。
	// 例如 packageName="advanced" 时,"advanced/pkg/xlog" -> "myproj/pkg/xlog",
	// 但不会改动注释里的 "advanced usage" 或字符串 "basic auth"。
	oldImport := "\"" + packageName + "/"
	newImport := "\"" + p.ProjectName + "/"
	err := filepath.Walk(p.ProjectName, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		newData := bytes.ReplaceAll(data, []byte(oldImport), []byte(newImport))
		if err := os.WriteFile(path, newData, 0644); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("walk file error: %w", err)
	}
	return nil
}

// handlerZip 将内嵌 zip 模板解压到 projectName 目录。
func handlerZip(projectName, templateName string) error {
	// 创建项目目录
	if err := os.MkdirAll(projectName, 0755); err != nil {
		return err
	}
	tempFile, err := tpl.NewTplZipFS.ReadFile(templateName + ".zip")
	if err != nil {
		return err
	}

	zipReader, err := zip.NewReader(bytes.NewReader(tempFile), int64(len(tempFile)))
	if err != nil {
		return err
	}

	// 遍历 zip 包里的文件
	baseAbs, err := filepath.Abs(projectName)
	if err != nil {
		return err
	}
	for _, file := range zipReader.File {
		// 防 zip slip：拒绝任何逃逸出目标目录的路径
		name := filepath.Clean(filepath.FromSlash(file.Name))
		if strings.HasPrefix(name, ".."+string(filepath.Separator)) || filepath.IsAbs(name) {
			return fmt.Errorf("zip entry %q has unsafe path", file.Name)
		}
		dstPath := filepath.Join(projectName, name)
		// 防 zip slip:解析为绝对路径后,必须仍在 baseAbs 之下。
		absPath, err := filepath.Abs(dstPath)
		if err != nil {
			return err
		}
		absPath = filepath.Clean(absPath)
		if absPath != baseAbs && !strings.HasPrefix(absPath, baseAbs+string(filepath.Separator)) {
			return fmt.Errorf("zip entry %q escapes target directory", file.Name)
		}
		fileMode := file.Mode()
		// 如果是目录，就创建目录
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(dstPath, fileMode); err != nil {
				return err
			}
			continue
		}

		// 获取 Reader
		fr, err := file.Open()
		if err != nil {
			return err
		}

		// 创建目标文件
		fw, err := os.OpenFile(dstPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, fileMode)
		if err != nil {
			_ = fr.Close()
			return err
		}

		_, copyErr := io.Copy(fw, fr)
		// M5: 同时检查写关闭的错误（磁盘满时 Close 可能携带错误）
		closeWErr := fw.Close()
		_ = fr.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeWErr != nil {
			return fmt.Errorf("close %s: %w", dstPath, closeWErr)
		}
	}

	return nil
}
