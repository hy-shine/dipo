# Dipo 原生取色器

`make build` 将 Swift helper 打包进 Dipo。它使用 ScreenCaptureKit 捕获每块显示器后，在独立非激活面板内显示冻结画面；点击不会传给后面的应用。屏幕画面只驻内存，不保存到历史。

- 移动鼠标：定位像素；方向键：移动一个捕获像素；Return/点击：确认；Esc/右键：取消。
- 输出 stdout JSON：`picked` 带 sRGB 分量和全局左上原点的 point 坐标；`cancelled`/`permission` 代表没有选色。错误写 stderr 并以非零状态结束。
- `resources/dipo-sampler --check-permission` 只检查当前屏幕录制权限，不请求。
- `make check-native` 运行合成图检查，并生成 `build/native-checks/picker-native.png`。这只检查像素映射和绘图，不冒充真实鼠标取色验收。
- `resources/dipo-sampler --capture-check` 在已有权限时短暂显示四个已知 sRGB 色块，真实捕获并比较中心值；无权限直接报告 `permission`，不请求。结果 `passed` 允许每通道最多 1/255 的捕获误差；程序不会自动改授权。

捕获采用半浮点线性 Display P3 数据后转换为首版的 sRGB；这防止先量化到 8 位造成暗色偏移，不表示首版支持 P3/HDR 色值输出。显示器配置变化时取消本次采样，避免旧坐标图像取错像素。多屏和 Spaces 仍需要真实设备交互验收。
