// WiFi Captive Portal H5 - Modern Interaction Logic & Apple Aesthetics

// 1. 动态时间/系统主题自适应引擎
(function initTheme() {
    // 优先采用系统 prefers-color-scheme 配置，若无则使用时间自适应 (6:00 - 18:00 为 Light，其它为 Dark)
    const systemPrefersDark = window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    const hour = new Date().getHours();
    const isNightTime = hour < 6 || hour >= 18;

    if (systemPrefersDark) {
        document.documentElement.className = 'dark-theme';
    } else if (isNightTime) {
        document.documentElement.className = 'dark-theme';
    } else {
        document.documentElement.className = 'light-theme';
    }
})();

document.addEventListener("DOMContentLoaded", () => {
    // 1. 从 URL 获取认证所需参数
    const params = new URLSearchParams(window.location.search);
    let hotelIdStr = params.get("hotelId") || params.get("hotel_id"); // 兼容下划线
    if (!hotelIdStr) {
        const pathParts = window.location.pathname.split('/');
        const hotelIdx = pathParts.indexOf('hotel');
        if (hotelIdx !== -1 && pathParts.length > hotelIdx + 1) {
            hotelIdStr = pathParts[hotelIdx + 1];
        }
    }
    const ip = params.get("ip") || params.get("wlanuserip") || "";
    const mac = params.get("mac") || "";
    const clientUrl = params.get("url") || params.get("dst") || "https://www.baidu.com";

    // 2. 映射页面元素
    const hotelNameEl = document.getElementById("hotel-name");
    const welcomeTextEl = document.getElementById("welcome-text");
    const phoneInput = document.getElementById("phone");
    const codeInput = document.getElementById("code");
    const btnSendCode = document.getElementById("btn-send-code");
    const btnConnect = document.getElementById("btn-connect");
    
    const alertError = document.getElementById("alert-error");
    const errorMsg = document.getElementById("error-msg");
    const alertSuccess = document.getElementById("alert-success");
    const successMsg = document.getElementById("success-msg");

    // === WebOTP API 自动读取并填入短信验证码 (同时支持 Android & iOS 14+ WebOTP API) ===
    if ('OTPCredential' in window) {
        const ac = new AbortController();
        navigator.credentials.get({
            otp: { transport: ['sms'] },
            signal: ac.signal
        }).then(otp => {
            if (otp && otp.code) {
                codeInput.value = otp.code;
                syncIosHeaderActionState();
                showAlert("success", "已为您自动提取并填入短信验证码！");
                // 自动点击连网放行
                setTimeout(() => {
                    btnConnect.click();
                }, 600);
            }
        }).catch(err => {
            console.log("WebOTP API 监听取消或暂不支持: ", err);
        });

        // 页面关闭或切走时释放 WebOTP 信号，防止内存泄露
        window.addEventListener("unload", () => {
            ac.abort();
        });
    }

    // iOS 顶栏拟态交互映射
    const navCancelBtn = document.querySelector(".ios-header-cancel");
    const navActionBtn = document.getElementById("nav-connect-text");

    let hotelId = 0;
    if (hotelIdStr) {
        hotelId = parseInt(hotelIdStr, 10);
    }

    // 3. 安全检查与防刷冷却状态记录
    let cooldownTimer = null;

    function showAlert(type, message) {
        if (type === "error") {
            alertSuccess.style.display = "none";
            errorMsg.textContent = message;
            alertError.style.display = "flex";
        } else {
            alertError.style.display = "none";
            successMsg.textContent = message;
            alertSuccess.style.display = "flex";
        }
    }

    function clearAlerts() {
        alertError.style.display = "none";
        alertSuccess.style.display = "none";
    }

    // 联动 iOS 顶栏 “继续” 按钮状态与表单校验
    function syncIosHeaderActionState() {
        const phone = phoneInput.value.trim();
        const code = codeInput.value.trim();
        const isPhoneValid = /^1[3-9]\d{9}$/.test(phone);
        const isCodeValid = /^\d{6}$/.test(code);

        if (isPhoneValid && isCodeValid) {
            navActionBtn.classList.remove("disabled");
        } else {
            navActionBtn.classList.add("disabled");
        }
    }

    phoneInput.addEventListener("input", syncIosHeaderActionState);
    codeInput.addEventListener("input", syncIosHeaderActionState);

    // 取消按钮重置表单
    if (navCancelBtn) {
        navCancelBtn.addEventListener("click", () => {
            phoneInput.value = "";
            codeInput.value = "";
            clearAlerts();
            syncIosHeaderActionState();
        });
    }

    // iOS 顶栏继续按钮连击主登录
    if (navActionBtn) {
        navActionBtn.addEventListener("click", () => {
            if (!navActionBtn.classList.contains("disabled") && !btnConnect.disabled) {
                btnConnect.click();
            }
        });
    }

    // 4. 从后端获取当前酒店 Portal 专属配置
    if (hotelId > 0) {
        fetch(`/portal/config?hotelId=${hotelId}`)
            .then(res => {
                if (!res.ok) {
                    throw new Error("加载酒店配置失败");
                }
                return res.json();
            })
            .then(data => {
                hotelNameEl.textContent = data.name || "WiFi 尊享连接";
                welcomeTextEl.textContent = data.welcome_text || "连入专享无线网络";
                
                // 动态将浏览器标签页标题更改为：<酒店名称>-无线网络认证接入
                document.title = (data.name || "xx酒店") + "-无线网络认证接入";

                if (data.status === 0) {
                    showAlert("error", "当前网关设备已被停用，请联系前台管理人员。");
                    btnConnect.disabled = true;
                    btnSendCode.disabled = true;
                    return;
                }

                // 判断当前酒店是否开启了“二次免登快捷上网”功能
                if (data.bypass_auth === 1) {
                    const savedPhone = localStorage.getItem(`wifi_bypass_phone_${hotelId}`) || getCookieValue(`wifi_bypass_phone_${hotelId}`);
                    const savedToken = localStorage.getItem(`wifi_bypass_token_${hotelId}`) || getCookieValue(`wifi_bypass_token_${hotelId}`);
                    
                    if (savedPhone && savedToken) {
                        phoneInput.value = savedPhone;
                        syncIosHeaderActionState();
                        
                        showAlert("success", "已识别您此前已认证成功，正在为您免短信无缝联网放行...");
                        btnConnect.disabled = true;
                        btnSendCode.disabled = true;
                        
                        setTimeout(() => {
                            autoBypassConnect(savedPhone, savedToken);
                        }, 1200);
                    }
                }
            })
            .catch(err => {
                showAlert("error", "无法连通认证网关，请检查无线网络连接是否稳定。");
            });
    } else {
        showAlert("error", "网关缺少商户参数 (hotelId)，连接已被系统拒绝。");
        btnConnect.disabled = true;
        btnSendCode.disabled = true;
    }

    // 5. 验证码发送事件处理
    btnSendCode.addEventListener("click", () => {
        const phone = phoneInput.value.trim();
        if (!/^1[3-9]\d{9}$/.test(phone)) {
            showAlert("error", "请输入正确的11位中国手机号码");
            return;
        }

        clearAlerts();
        btnSendCode.disabled = true;
        btnSendCode.textContent = "正在发送...";
        
        // 利用用户直接点击的同步手势上下文，完美绕过浏览器安全限制，100% 唤起软键盘
        codeInput.focus();

        fetch("/portal/sms/send", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                hotelId: hotelId,
                phone: phone,
                ip: ip,
                mac: mac
            })
        })
        .then(async res => {
            const data = await res.json();
            if (!res.ok) {
                throw new Error(data.error || "短信发送失败");
            }
            showAlert("success", data.message || "验证码发送成功");
            startCooldown(60);
            codeInput.focus();
        })
        .catch(err => {
            showAlert("error", err.message);
            btnSendCode.disabled = false;
            btnSendCode.textContent = "获取验证码";
        });
    });

    // 冷却倒计时辅助
    function startCooldown(seconds) {
        let remaining = seconds;
        btnSendCode.disabled = true;
        btnSendCode.textContent = `${remaining}秒后重新获取`;

        cooldownTimer = setInterval(() => {
            remaining--;
            if (remaining <= 0) {
                clearInterval(cooldownTimer);
                btnSendCode.disabled = false;
                btnSendCode.textContent = "获取验证码";
            } else {
                btnSendCode.textContent = `${remaining}秒后重新获取`;
            }
        }, 1000);
    }

    // 6. 验证并放行连网请求
    btnConnect.addEventListener("click", () => {
        const phone = phoneInput.value.trim();
        const code = codeInput.value.trim();

        if (!/^1[3-9]\d{9}$/.test(phone)) {
            showAlert("error", "请输入正确的手机号码");
            return;
        }
        if (!/^\d{6}$/.test(code)) {
            showAlert("error", "请输入6位数字验证码");
            return;
        }

        clearAlerts();
        btnConnect.disabled = true;
        btnConnect.textContent = "正在登录无线网络...";

        fetch("/portal/sms/verify", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                hotelId: hotelId,
                phone: phone,
                code: code,
                ip: ip,
                mac: mac,
                client_url: clientUrl
            })
        })
        .then(async res => {
            const data = await res.json();
            if (!res.ok) {
                throw new Error(data.error || "连接验证码失败");
            }
            showAlert("success", "验证成功！正在跳转至目标网络，请稍候...");
            
            // 如果后端返回了免登凭证，则将其写入本地 localStorage 和加密 Cookie，有效期 30 天
            if (data.bypass_token && data.phone) {
                localStorage.setItem(`wifi_bypass_phone_${hotelId}`, data.phone);
                localStorage.setItem(`wifi_bypass_token_${hotelId}`, data.bypass_token);
                
                const expires = new Date();
                expires.setTime(expires.getTime() + (30 * 24 * 60 * 60 * 1000));
                document.cookie = `wifi_bypass_phone_${hotelId}=${data.phone};expires=${expires.toUTCString()};path=/`;
                document.cookie = `wifi_bypass_token_${hotelId}=${data.bypass_token};expires=${expires.toUTCString()};path=/`;
            }

            // 收到后端计算好、算过签名的链接后，客户端直接重定向，彻底根除 RFC Strict Nginx 400 报错
            setTimeout(() => {
                window.location.href = data.redirect_url;
            }, 1000);
        })
        .catch(err => {
            showAlert("error", err.message);
            btnConnect.disabled = false;
            btnConnect.textContent = "立即尊享高速上网";
        });
    });

    // 辅助与快捷免密认证函数
    function getCookieValue(name) {
        const value = `; ${document.cookie}`;
        const parts = value.split(`; ${name}=`);
        if (parts.length === 2) return parts.pop().split(';').shift();
        return "";
    }

    function autoBypassConnect(phone, token) {
        fetch("/portal/sms/bypass", {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({
                hotelId: hotelId,
                phone: phone,
                bypass_token: token,
                ip: ip,
                mac: mac,
                client_url: clientUrl
            })
        })
        .then(async res => {
            const data = await res.json();
            if (!res.ok) {
                throw new Error(data.error || "快捷免登授权令牌失效");
            }
            showAlert("success", "快速免验证连接成功！正在为您重定向至网络...");
            setTimeout(() => {
                window.location.href = data.redirect_url;
            }, 1000);
        })
        .catch(err => {
            // 清除无效或过期的凭证引导用户回落到传统短信验证
            localStorage.removeItem(`wifi_bypass_phone_${hotelId}`);
            localStorage.removeItem(`wifi_bypass_token_${hotelId}`);
            document.cookie = `wifi_bypass_phone_${hotelId}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
            document.cookie = `wifi_bypass_token_${hotelId}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
            
            showAlert("error", "免登连接授权已失效，请在此处重新输入手机号获取验证码。");
            btnConnect.disabled = false;
            btnSendCode.disabled = false;
            phoneInput.value = "";
            syncIosHeaderActionState();
        });
    }
});
