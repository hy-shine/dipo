import AppKit

struct Sample: Encodable {
    let status: String
    let red: Double
    let green: Double
    let blue: Double
    let x: Double
    let y: Double

    static func empty(_ status: String) -> Sample {
        Sample(status: status, red: 0, green: 0, blue: 0, x: 0, y: 0)
    }
}

@MainActor
final class SamplerDelegate: NSObject, NSApplicationDelegate {
    private var panels: [PickerPanel] = []
    private var views: [PickerView] = []
    private var activeView: PickerView?
    private var finished = false
    private var screenObserver: NSObjectProtocol?

    func applicationDidFinishLaunching(_ notification: Notification) {
        if CommandLine.arguments.contains("--check-permission") {
            finish(.empty(CGPreflightScreenCaptureAccess() ? "ready" : "permission"))
            return
        }
        if CommandLine.arguments.contains("--capture-check") {
            Task {
                do {
                    let data = try await runCaptureChecks()
                    FileHandle.standardOutput.write(data)
                    NSApp.terminate(nil)
                } catch { fail(error) }
            }
            return
        }
        if !CGPreflightScreenCaptureAccess(), !CGRequestScreenCaptureAccess() {
            finish(.empty("permission"))
            return
        }
        Task {
            do {
                let snapshots = try await captureDisplays()
                show(snapshots)
            } catch {
                if !CGPreflightScreenCaptureAccess() {
                    finish(.empty("permission"))
                } else {
                    fail(error)
                }
            }
        }
    }

    private func show(_ snapshots: [FrozenDisplay]) {
        for snapshot in snapshots {
            let panel = PickerPanel(snapshot: snapshot)
            let view = PickerView(snapshot: snapshot)
            view.owner = self
            panel.contentView = view
            panel.makeFirstResponder(view)
            panels.append(panel)
            views.append(view)
        }
        for panel in panels { panel.orderFrontRegardless() }
        let point = NSEvent.mouseLocation
        if let view = views.first(where: { $0.snapshot.frame.contains(point) }) ?? views.first {
            moved(to: view, point: point)
        }
        screenObserver = NotificationCenter.default.addObserver(forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.cancel() }
        }
    }

    func moved(to view: PickerView, point: CGPoint) {
        if activeView !== view {
            activeView?.selection = nil
            activeView?.needsDisplay = true
            activeView = view
            view.window?.makeKey()
        }
        view.selection = view.snapshot.position(at: point)
        view.needsDisplay = true
    }

    func nudge(columns: Int, rows: Int) {
        guard let view = activeView, let selection = view.selection else { return }
        view.selection = view.snapshot.nudge(selection, columns: columns, rows: rows)
        view.needsDisplay = true
    }

    func commit() {
        guard let view = activeView, let selection = view.selection else { return }
        let pixel = view.snapshot.pixel(at: selection)
        let point = view.snapshot.point(at: selection)
        let top = NSScreen.screens.first?.frame.maxY ?? 0
        finish(Sample(status: "picked", red: Double(pixel.red) / 255, green: Double(pixel.green) / 255, blue: Double(pixel.blue) / 255, x: Double(point.x), y: Double(top - point.y)))
    }

    func cancel() { finish(.empty("cancelled")) }

    private func cleanUp() {
        if let screenObserver { NotificationCenter.default.removeObserver(screenObserver) }
        screenObserver = nil
        for panel in panels { panel.orderOut(nil) }
        panels.removeAll()
        views.removeAll()
        activeView = nil
        NSCursor.arrow.set()
    }

    private func finish(_ sample: Sample) {
        guard !finished else { return }
        finished = true
        cleanUp()
        do {
            let data = try JSONEncoder().encode(sample)
            FileHandle.standardOutput.write(data)
            NSApp.terminate(nil)
        } catch { fail(error) }
    }

    private func fail(_ error: Error) {
        cleanUp()
        FileHandle.standardError.write(Data("screen capture: \(error.localizedDescription)\n".utf8))
        exit(EXIT_FAILURE)
    }
}
