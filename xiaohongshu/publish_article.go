package xiaohongshu

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
)

// PublishArticleContent 发布长文章内容
type PublishArticleContent struct {
	Title       string
	Content     string
	Tags        []string // 标签列表，与内容分开
	ImagePaths  []string
	PublishTime string // 可选的定时发布时间，格式: "2025-09-12 14:22"（北京时间）
}

type PublishArticleAction struct {
	page *rod.Page
}

const (
	urlOfArticlePublish = `https://creator.xiaohongshu.com/publish/publish?source=official&from=menu&target=article`
)

func NewPublishArticleAction(page *rod.Page) (*PublishArticleAction, error) {
	pp := page.Timeout(60 * time.Second)

	pp.MustNavigate(urlOfArticlePublish)
	pp.MustWaitLoad()

	slog.Info("导航到文章发布页面，等待页面加载完成")

	// 等待页面完全加载和渲染
	time.Sleep(5 * time.Second)

	// 使用正则表达式匹配包含"新的创作"的span元素
	found := false
	for attempt := 0; attempt < 2; attempt++ {
		slog.Info("尝试查找'新的创作'按钮", "attempt", attempt+1)
		
		// 使用MustElementR正则匹配
		elem, err := pp.Timeout(10*time.Second).ElementR("span", "新的创作")
		
		if err != nil {
			slog.Warn("使用正则查找失败", "error", err, "attempt", attempt+1)
			if attempt == 0 {
				slog.Info("第一次查找失败，刷新页面")
				pp.MustReload()
				pp.MustWaitLoad()
				time.Sleep(5 * time.Second)
			}
			continue
		}
		
		// 找到元素
		text, _ := elem.Text()
		slog.Info("找到'新的创作'按钮", "text", text)
		found = true
		
		// 等待1秒后点击
		time.Sleep(1 * time.Second)
		if err := elem.Click(proto.InputMouseButtonLeft, 1); err != nil {
			slog.Error("点击'新的创作'失败", "error", err)
			return nil, errors.Wrap(err, "点击'新的创作'失败")
		}
		slog.Info("已点击'新的创作'按钮")
		break
	}

	if !found {
		return nil, errors.New("无法找到'新的创作'按钮，请检查页面是否正确加载")
	}

	// 等待页面加载，寻找creator-container
	pp.MustElement(".creator-container").MustWaitVisible()
	slog.Info("文章编辑器加载成功")

	// 额外等待确保页面完全加载
	time.Sleep(1 * time.Second)

	return &PublishArticleAction{
		page: pp,
	}, nil
}

func (p *PublishArticleAction) Publish(ctx context.Context, content PublishArticleContent) error {
	if len(content.ImagePaths) == 0 {
		return errors.New("图片不能为空")
	}

	page := p.page.Context(ctx)

	// 输入标题
	if err := inputTitle(page, content.Title); err != nil {
		return errors.Wrap(err, "输入标题失败")
	}

	// 输入正文内容
	if err := inputMainContent(page, content.Content); err != nil {
		return errors.Wrap(err, "输入正文内容失败")
	}

	// 点击一键排版
	if err := clickAutoFormat(page); err != nil {
		return errors.Wrap(err, "点击一键排版失败")
	}

	// 选择模板
	if err := selectTemplate(page); err != nil {
		return errors.Wrap(err, "选择模板失败")
	}

	// 点击下一步
	if err := clickNextStep(page); err != nil {
		return errors.Wrap(err, "点击下一步失败")
	}

	// 上传图片
	if err := uploadArticleImages(page, content.ImagePaths); err != nil {
		return errors.Wrap(err, "上传图片失败")
	}

	// 输入标签
	if err := inputTags(page, content.Tags); err != nil {
		return errors.Wrap(err, "输入标签失败")
	}

	// 设置定时发布（如果提供）
	if content.PublishTime != "" {
		if err := setScheduledPublish(page, content.PublishTime); err != nil {
			return errors.Wrap(err, "设置定时发布失败")
		}
	}

	// 提交发布
	if err := submitArticlePublish(page); err != nil {
		return errors.Wrap(err, "提交发布失败")
	}

	return nil
}

// inputTitle 输入标题
func inputTitle(page *rod.Page, title string) error {
	slog.Info("开始输入标题", "title", title)
	
	titleInput := page.MustElement(`textarea.d-text[placeholder="输入标题"]`)
	titleInput.MustClick()
	titleInput.MustInput(title)
	
	time.Sleep(1 * time.Second)
	slog.Info("标题输入完成")
	return nil
}

// inputMainContent 输入正文内容
func inputMainContent(page *rod.Page, content string) error {
	slog.Info("开始输入正文内容")
	
	// 查找可编辑的div
	contentDiv := page.MustElement(`div.tiptap.ProseMirror[contenteditable="true"]`)
	contentDiv.MustClick()
	contentDiv.MustInput(content)
	
	time.Sleep(1 * time.Second)
	slog.Info("正文内容输入完成")
	return nil
}

// clickAutoFormat 点击一键排版
func clickAutoFormat(page *rod.Page) error {
	slog.Info("点击一键排版")
	
	// 查找"一键排版"按钮
	spans, err := page.Elements(`span.next-btn-text`)
	if err != nil {
		return errors.Wrap(err, "未找到一键排版按钮")
	}
	
	for _, span := range spans {
		text, err := span.Text()
		if err != nil {
			continue
		}
		if strings.Contains(text, "一键排版") {
			if err := span.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return errors.Wrap(err, "点击一键排版失败")
			}
			slog.Info("已点击一键排版")
			time.Sleep(5 * time.Second)
			return nil
		}
	}
	
	return errors.New("未找到'一键排版'按钮")
}

// selectTemplate 选择模板
func selectTemplate(page *rod.Page) error {
	slog.Info("开始选择模板")
	
	// 查找tab-panel
	tabPanel := page.MustElement(".tab-panel")
	slog.Info("找到tab-panel")
	
	// 滚动tab-panel
	slog.Info("开始滚动查找模板")
	maxScrollAttempts := 10
	for i := 0; i < maxScrollAttempts; i++ {
		// 查找"轻感明快"模板
		spans, err := tabPanel.Elements(`span.template-title`)
		if err == nil {
			for _, span := range spans {
				text, err := span.Text()
				if err != nil {
					continue
				}
				if text == "轻感明快" {
					slog.Info("找到'轻感明快'模板")
					if err := span.Click(proto.InputMouseButtonLeft, 1); err != nil {
						return errors.Wrap(err, "点击模板失败")
					}
					time.Sleep(5 * time.Second)
					slog.Info("模板选择完成")
					return nil
				}
			}
		}
		
		// 如果未找到，滚动600px
		slog.Info("未找到模板，继续滚动", "attempt", i+1)
		tabPanel.MustEval(`() => this.scrollTop += 600`)
		time.Sleep(500 * time.Millisecond)
	}
	
	return errors.New("未找到'轻感明快'模板")
}

// clickNextStep 点击下一步
func clickNextStep(page *rod.Page) error {
	slog.Info("点击下一步")
	
	spans, err := page.Elements(`span.d-text`)
	if err != nil {
		return errors.Wrap(err, "未找到下一步按钮")
	}
	
	for _, span := range spans {
		text, err := span.Text()
		if err != nil {
			continue
		}
		if text == "下一步" {
			if err := span.Click(proto.InputMouseButtonLeft, 1); err != nil {
				return errors.Wrap(err, "点击下一步失败")
			}
			slog.Info("已点击下一步")
			time.Sleep(5 * time.Second)
			return nil
		}
	}
	
	return errors.New("未找到'下一步'按钮")
}

// uploadArticleImages 上传图片
func uploadArticleImages(page *rod.Page, imagePaths []string) error {
	slog.Info("开始上传图片", "count", len(imagePaths))
	
	// 查找添加按钮（带有加号图标的div）
	addButton, err := page.Element(`div.entry`)
	if err != nil {
		return errors.Wrap(err, "未找到添加图片按钮")
	}
	
	// 点击添加按钮
	if err := addButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return errors.Wrap(err, "点击添加按钮失败")
	}
	
	slog.Info("已点击添加按钮，等待上传控件")
	time.Sleep(1 * time.Second)
	
	// 查找文件上传输入框
	uploadInput, err := page.Element(`input[type="file"]`)
	if err != nil {
		return errors.Wrap(err, "未找到文件上传输入框")
	}
	
	// 上传所有图片
	uploadInput.MustSetFiles(imagePaths...)
	slog.Info("文件已设置到上传输入框")
	
	// 等待上传完成
	time.Sleep(5 * time.Second)
	
	slog.Info("图片上传完成")
	return nil
}

// inputTags 输入标签
func inputTags(page *rod.Page, tags []string) error {
	if len(tags) == 0 {
		slog.Info("没有标签需要输入")
		return nil
	}
	
	slog.Info("开始输入标签", "tags", tags)
	
	// 查找正文描述输入框
	descDiv, err := page.Element(`div.tiptap.ProseMirror[contenteditable="true"][role="textbox"]`)
	if err != nil {
		return errors.Wrap(err, "未找到标签输入框")
	}
	
	// 点击输入框获得焦点
	descDiv.MustClick()
	time.Sleep(500 * time.Millisecond)
	
	// 逐个输入标签
	for i, tag := range tags {
		// 确保标签以#开头
		if !strings.HasPrefix(tag, "#") {
			tag = "#" + tag
		}
		
		slog.Info("输入标签", "tag", tag)
		
		// 如果不是第一个标签，添加空格分隔
		if i > 0 {
			descDiv.MustInput(" ")
		}
		
		// 输入标签
		descDiv.MustInput(tag)
		
		// 等待话题选择弹窗出现
		time.Sleep(2 * time.Second)
		
		// 尝试选择话题
		if err := selectTopicFromPopup(page); err != nil {
			slog.Warn("选择话题失败，继续处理", "tag", tag, "error", err)
		}
	}
	
	slog.Info("标签输入完成")
	return nil
}

// submitArticlePublish 提交发布
func submitArticlePublish(page *rod.Page) error {
	slog.Info("提交发布")
	
	// 查找发布按钮
	submitButton := page.MustElement("div.submit div.d-button-content")
	submitButton.MustClick()
	
	time.Sleep(3 * time.Second)
	slog.Info("发布完成")
	
	return nil
}
