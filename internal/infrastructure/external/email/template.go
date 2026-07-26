package email

import "fmt"

// htmlTemplate 验证码邮件的 HTML 骨架（与原 email_service.getHtmlTemplate 逐字一致，
// 含原模板中仅有缩进的空行，保证渲染出的正文字节与原实现完全相同）。
const htmlTemplate = `
<div style="font-family: 'MiSans', 'Noto Sans SC', 'Helvetica Neue', Helvetica, Arial, sans-serif; box-sizing: border-box; width: 100%%; margin: 0; padding: 0; background-color: #e8e8e8; background-image: url('%s'); background-size: cover; background-position: center; background-repeat: no-repeat;">
    <div style="background-color: rgba(0,0,0,0.1); width: 100%%; min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 20px 0;">
        <table width="100%%" cellpadding="0" cellspacing="0" style="margin: 0; padding: 0;">
            <tr>
                <td></td>
                <td style="display: block !important; max-width: 600px !important; margin: 0 auto !important; clear: both !important;" width="600">
                    <div style="max-width: 600px; margin: 0 auto; display: block; padding: 20px;">
                        <table style="background: rgba(255, 255, 255, 0.85); border: 1px solid rgba(233, 233, 233, 0.5); border-radius: 12px; backdrop-filter: blur(5px); -webkit-backdrop-filter: blur(5px); box-shadow: 0 4px 20px rgba(0,0,0,0.1);" width="100%%" cellpadding="0" cellspacing="0">
                            <tr>
                                <td style="padding: 0;">
                                    <table width="100%%" cellpadding="0" cellspacing="0">
                                        <tr>
                                            <td style="padding: 20px 30px; text-align: center; background: linear-gradient(to right, #00a1d6, #fb7299); border-radius: 12px 12px 0 0; color: #ffffff;">
                                                <img src="%s" alt="Logo" style="width: 32px; height: 32px; vertical-align: middle; margin-right: 10px;">
                                                <h1 style="font-size: 26px; color: #ffffff; margin: 0; font-weight: 600; display: inline-block; vertical-align: middle;">【%s】%s</h1>
                                            </td>
                                        </tr>
                                        <tr>
                                            <td style="padding: 30px;">
                                                <table width="100%%" cellpadding="0" cellspacing="0">
                                                    <tr>
                                                        <td style="padding: 0;">
                                                            <p style="font-size: 17px; color: #444; line-height: 1.8; margin: 0 0 20px;">
                                                                亲爱的用户，<br>%s
                                                            </p>
                                                            
                                                            <div style="text-align: center; margin-bottom: 30px;">
                                                                <span style="display: inline-block; background: linear-gradient(to right, #fb7299, #f755a9); border: 2px solid #ffb3c1; border-radius: 10px; padding: 18px 35px; font-size: 38px; font-weight: bold; color: #ffffff; letter-spacing: 4px; text-shadow: 1px 1px 3px rgba(0,0,0,0.1);">
                                                                    %s
                                                                </span>
                                                            </div>
                                                            
                                                            <p style="font-size: 15px; color: #777; line-height: 1.7; margin: 0 0 25px;">%s</p>
                                                            
                                                            <div style="text-align: center; margin-bottom: 25px;">
                                                                <img src="%s" alt="Character" style="max-width: 120px; border-radius: 50%%; border: 3px solid #00a1d6; box-shadow: 0 2px 10px rgba(0,0,0,0.1); margin: 0 auto; display: block;">
                                                            </div>

                                                            <p style="font-size: 15px; color: #777; line-height: 1.7; margin: 0;">如果这不是您本人的操作，请务必立即修改密码并联系客服！</p>
                                                        </td>
                                                    </tr>
                                                </table>
                                            </td>
                                        </tr>
                                    </table>
                                </td>
                            </tr>
                        </table>
                        <div style="text-align: center; font-size: 13px; color: rgba(255,255,255,0.8); margin-top: 25px; text-shadow: 1px 1px 2px rgba(0,0,0,0.3);">
                            <p style="margin: 0 0 8px;">本邮件由系统自动发送，请勿直接回复哦。</p>
                            <p style="margin: 0;">&copy; %s. All Rights Reserved.</p>
                        </div>
                    </div>
                </td>
                <td></td>
            </tr>
        </table>
    </div>
</div>
`

// buildHTMLTemplate 注入动态内容生成验证码邮件正文。
//
// nickname: 发送者昵称 (例如: 哔哩哔哩)
// mainTitle: 邮件主标题 (例如: 账号注册)
// introText: 验证码上方的介绍文字
// code: 验证码
// noteText: 验证码下方的提示文字
// bgImageURL: 邮件背景图片URL
// logoIconURL: 顶部Logo图标URL (例如B站小电视)
// characterImageURL: 内容区域角色图片URL.
func buildHTMLTemplate(
	nickname, mainTitle, introText, code, noteText, bgImageURL, logoIconURL, characterImageURL string,
) string {
	return fmt.Sprintf(
		htmlTemplate,
		bgImageURL,
		logoIconURL,
		nickname,
		mainTitle,
		introText,
		code,
		noteText,
		characterImageURL,
		nickname,
	)
}
