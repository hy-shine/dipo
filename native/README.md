# Dipo 原生取色器

`make build` 将 Swift helper 打包进 Dipo。它使用 ScreenCaptureKit 捕获每块显示器后，在独立非激活面板内显示冻结画面；点击不会传给后面的应用。屏幕画面只驻内存，不保存到历史。

- 移动鼠标：定位像素；方向键：移动一个捕获像素；Return/点击：确认；Esc/右键：取消。
- 输出 stdout JSON：`picked` 带 extended sRGB 分量（sRGB 显示不了的颜色会小于 0 或大于 1，不截断）和全局左上原点的 point 坐标；`cancelled`/`permission` 代表没有选色。错误写 stderr 并以非零状态结束。
- `resources/dipo-sampler --check-permission` 只检查当前屏幕录制权限，不请求。
- `make check-native` 运行合成图检查，并生成 `build/native-checks/picker-native.png`。这只检查像素映射和绘图，不冒充真实鼠标取色验收。
- `resources/dipo-sampler --capture-check` 在已有权限时短暂显示四个已知 sRGB 色块，显示器能显示 Display P3 时（结果里 `wideGamut` 为 true）再加三个超出 sRGB 的 Display P3 色块，真实捕获并比较中心值；无权限直接报告 `permission`，不请求。期望值是 ColorSync 把色块换算成的 extended sRGB 分量。结果 `passed` 要求每通道误差最多 1/255，且每个色块是否超出 sRGB 与画上去的颜色一致；程序不会自动改授权。屏幕锁定时捕获到的是锁屏画面，结果是 `mismatch`。

捕获采用半浮点线性 Display P3 数据。冻结画面转成 8 位 sRGB 来画；放大镜和取到的颜色从原始捕获读，只换算读到的像素，换成 32 位浮点 extended sRGB，所以 sRGB 外的颜色保留真实分量，不先量化到 8 位，也不多存一份浮点全屏副本。分量四舍五入到 8 位不需要截断时算在 sRGB 内，Go 的 `internal/colour` 用同一条线。显示器配置变化时取消本次采样，避免旧坐标图像取错像素。多屏和 Spaces 仍需要真实设备交互验收。
