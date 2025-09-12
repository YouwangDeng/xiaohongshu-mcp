package xiaohongshu

import (
	"context"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishImageContent 发布图文内容
type PublishImageContent struct {
	Title       string
	Content     string
	ImagePaths  []string
	PublishTime string // 可选的定时发布时间，格式: "2025-09-12 14:22"（北京时间）
}

type PublishAction struct {
	page *rod.Page
}

const (
	urlOfPublic = `https://creator.xiaohongshu.com/publish/publish?source=official`
)

func NewPublishImageAction(page *rod.Page) (*PublishAction, error) {

	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate(urlOfPublic)

	pp.MustElement(`div.upload-content`).MustWaitVisible()
	slog.Info("wait for upload-content visible success")

	// 等待一段时间确保页面完全加载
	time.Sleep(1 * time.Second)

	createElems := pp.MustElements("div.creator-tab")
	slog.Info("foundcreator-tab elements", "count", len(createElems))
	for _, elem := range createElems {
		text, err := elem.Text()
		if err != nil {
			slog.Error("获取元素文本失败", "error", err)
			continue
		}

		if text == "上传图文" {
			if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
				slog.Error("点击元素失败", "error", err)
				continue
			}
			break
		}
	}

	time.Sleep(1 * time.Second)

	return &PublishAction{
		page: pp,
	}, nil
}

func (p *PublishAction) Publish(ctx context.Context, content PublishImageContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	if err := uploadImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "小红书上传图片失败")
	}

	if err := submitPublish(page, content.Title, content.Content, content.PublishTime); err != nil {
		return errors.Wrap(err, "小红书发布失败")
	}

	return nil
}

func uploadImages(page *rod.Page, imagesPaths []string) error {
	pp := page.Timeout(30 * time.Second)

	slog.Info("开始上传图片", "paths", imagesPaths)

	// 等待上传输入框出现
	uploadInput := pp.MustElement(".upload-input")
	slog.Info("找到上传输入框")

	// 上传多个文件
	uploadInput.MustSetFiles(imagesPaths...)
	slog.Info("文件已设置到上传输入框")

	// 等待上传完成，增加等待时间
	time.Sleep(5 * time.Second)

	// 检查是否有上传错误提示
	if hasError, err := checkUploadErrors(pp); err != nil {
		slog.Error("检查上传错误时出现问题", "error", err)
	} else if hasError {
		return errors.New("图片上传失败，请检查图片格式和大小")
	}

	slog.Info("图片上传完成")
	return nil
}

// checkUploadErrors 检查页面上是否有上传错误提示
func checkUploadErrors(page *rod.Page) (bool, error) {
	// 检查常见的错误提示元素
	errorSelectors := []string{
		".upload-error",
		".error-message", 
		".upload-failed",
		"[class*='error']",
		"[class*='fail']",
	}

	for _, selector := range errorSelectors {
		exists, _, err := page.Has(selector)
		if err != nil {
			continue // 忽略选择器错误，继续检查下一个
		}
		if exists {
			// 尝试获取错误信息
			if elem, err := page.Element(selector); err == nil {
				if text, err := elem.Text(); err == nil {
					slog.Error("发现上传错误", "selector", selector, "message", text)
					return true, nil
				}
			}
			return true, nil
		}
	}

	// 检查页面文本中是否包含失败相关的中文提示
	pageText, err := page.MustElement("body").Text()
	if err == nil {
		failureKeywords := []string{"上传失败", "文件上传失败", "部分文件上传失败", "格式不支持", "文件过大"}
		for _, keyword := range failureKeywords {
			if strings.Contains(pageText, keyword) {
				slog.Error("页面包含失败关键词", "keyword", keyword)
				return true, nil
			}
		}
	}

	return false, nil
}

func submitPublish(page *rod.Page, title, content, publishTime string) error {

	titleElem := page.MustElement("div.d-input input")
	titleElem.MustInput(title)

	time.Sleep(1 * time.Second)

	contentElem, ok := getContentElement(page)
	if !ok {
		return errors.New("没有找到内容输入框")
	}

	// 处理带话题的内容
	if err := inputContentWithTopics(page, contentElem, content); err != nil {
		return errors.Wrap(err, "输入内容和话题失败")
	}

	time.Sleep(1 * time.Second)

	// 如果提供了发布时间，设置定时发布
	if publishTime != "" {
		if err := setScheduledPublish(page, publishTime); err != nil {
			return errors.Wrap(err, "设置定时发布失败")
		}
	}

	submitButton := page.MustElement("div.submit div.d-button-content")
	submitButton.MustClick()

	time.Sleep(3 * time.Second)

	return nil
}

// setScheduledPublish 设置定时发布
func setScheduledPublish(page *rod.Page, publishTime string) error {
	slog.Info("设置定时发布", "publishTime", publishTime)
	
	// 查找并点击"定时发布"单选按钮
	scheduledRadio, err := page.Element("span.el-radio__label")
	if err != nil {
		return errors.Wrap(err, "未找到定时发布单选按钮")
	}
	
	// 检查是否是"定时发布"标签
	labelText, err := scheduledRadio.Text()
	if err != nil || labelText != "定时发布" {
		// 如果第一个不是，尝试查找所有的单选按钮标签
		radioLabels, err := page.Elements("span.el-radio__label")
		if err != nil {
			return errors.Wrap(err, "未找到单选按钮标签")
		}
		
		var foundScheduledRadio *rod.Element
		for _, label := range radioLabels {
			text, err := label.Text()
			if err != nil {
				continue
			}
			if text == "定时发布" {
				foundScheduledRadio = label
				break
			}
		}
		
		if foundScheduledRadio == nil {
			return errors.New("未找到'定时发布'单选按钮")
		}
		scheduledRadio = foundScheduledRadio
	}
	
	// 点击定时发布单选按钮
	if err := scheduledRadio.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击定时发布单选按钮失败")
	}
	
	slog.Info("已点击定时发布单选按钮")
	time.Sleep(1 * time.Second)
	
	// 查找时间输入框
	timeInput, err := page.Element("input.el-input__inner[placeholder*='选择日期和时间']")
	if err != nil {
		return errors.Wrap(err, "未找到时间输入框")
	}
	
	// 点击时间输入框并输入时间
	if err := timeInput.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击时间输入框失败")
	}
	
	// 清空输入框并输入新时间
	timeInput.MustSelectAllText()
	timeInput.MustInput(publishTime)
	
	slog.Info("已输入发布时间", "time", publishTime)
	
	// 点击输入框外部以确认时间选择
	page.MustElement("body").MustClick()
	time.Sleep(500 * time.Millisecond)
	
	return nil
}

// inputContentWithTopics 输入内容并处理话题选择
func inputContentWithTopics(page *rod.Page, contentElem *rod.Element, content string) error {
	slog.Info("开始输入内容并处理话题", "content", content)
	
	// 点击内容框获得焦点
	contentElem.MustClick()
	
	// 使用正则表达式找到所有话题
	topicRegex := regexp.MustCompile(`#([^\s#]+)`)
	topics := topicRegex.FindAllString(content, -1)
	
	// 移除原内容中的话题，得到纯文本
	contentWithoutTopics := topicRegex.ReplaceAllString(content, "")
	
	// 先输入纯文本内容
	slog.Info("输入纯文本内容", "content", contentWithoutTopics)
	contentElem.MustInput(contentWithoutTopics)
	
	// 如果有话题，在新行添加话题
	if len(topics) > 0 {
		// 添加两个换行符，创建空行分隔
		contentElem.MustInput("\n\n")
		
		// 逐个添加话题
		for i, topic := range topics {
			slog.Info("输入话题", "topic", topic)
			
			// 如果不是第一个话题，添加空格分隔
			if i > 0 {
				contentElem.MustInput(" ")
			}
			
			// 输入话题（包含#号）
			contentElem.MustInput(topic)
			
			// 等待话题选择弹窗出现
			time.Sleep(2 * time.Second)
			
			// 查找并点击话题选择容器中的第一个项目
			if err := selectTopicFromPopup(page); err != nil {
				slog.Warn("选择话题失败，继续处理", "topic", topic, "error", err)
				// 不返回错误，继续处理剩余话题
			}
		}
	}
	
	slog.Info("内容和话题输入完成")
	return nil
}

// selectTopicFromPopup 从话题选择弹窗中选择第一个话题
func selectTopicFromPopup(page *rod.Page) error {
	// 查找话题选择容器
	containerSelector := "#creator-editor-topic-container"
	
	// 等待容器出现，设置较短的超时时间
	container, err := page.Timeout(3 * time.Second).Element(containerSelector)
	if err != nil {
		return errors.Wrap(err, "话题选择容器未找到")
	}
	
	// 查找第一个话题项目
	itemSelectors := []string{
		".item.is-selected", // 优先选择已选中的
		".item:first-child", // 或者第一个项目
		".item",             // 或者任意项目
	}
	
	var selectedItem *rod.Element
	for _, selector := range itemSelectors {
		if item, err := container.Element(selector); err == nil {
			selectedItem = item
			break
		}
	}
	
	if selectedItem == nil {
		return errors.New("未找到可选择的话题项目")
	}
	
	// 点击选择话题
	if err := selectedItem.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击话题项目失败")
	}
	
	slog.Info("成功选择话题")
	
	// 等待弹窗消失
	time.Sleep(500 * time.Millisecond)
	
	return nil
}

// 查找内容输入框 - 使用Race方法处理两种样式
func getContentElement(page *rod.Page) (*rod.Element, bool) {
	var foundElement *rod.Element
	var found bool

	page.Race().
		Element("div.ql-editor").MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		ElementFunc(func(page *rod.Page) (*rod.Element, error) {
			return findTextboxByPlaceholder(page)
		}).MustHandle(func(e *rod.Element) {
		foundElement = e
		found = true
	}).
		MustDo()

	if found {
		return foundElement, true
	}

	slog.Warn("no content element found by any method")
	return nil, false
}

func findTextboxByPlaceholder(page *rod.Page) (*rod.Element, error) {
	elements := page.MustElements("p")
	if elements == nil {
		return nil, errors.New("no p elements found")
	}

	// 查找包含指定placeholder的元素
	placeholderElem := findPlaceholderElement(elements, "输入正文描述")
	if placeholderElem == nil {
		return nil, errors.New("no placeholder element found")
	}

	// 向上查找textbox父元素
	textboxElem := findTextboxParent(placeholderElem)
	if textboxElem == nil {
		return nil, errors.New("no textbox parent found")
	}

	return textboxElem, nil
}

func findPlaceholderElement(elements []*rod.Element, searchText string) *rod.Element {
	for _, elem := range elements {
		placeholder, err := elem.Attribute("data-placeholder")
		if err != nil || placeholder == nil {
			continue
		}

		if strings.Contains(*placeholder, searchText) {
			return elem
		}
	}
	return nil
}

func findTextboxParent(elem *rod.Element) *rod.Element {
	currentElem := elem
	for i := 0; i < 5; i++ {
		parent, err := currentElem.Parent()
		if err != nil {
			break
		}

		role, err := parent.Attribute("role")
		if err != nil || role == nil {
			currentElem = parent
			continue
		}

		if *role == "textbox" {
			return parent
		}

		currentElem = parent
	}
	return nil
}
