import AppKit

MainActor.assumeIsolated {
    let application = NSApplication.shared
    application.setActivationPolicy(.accessory)
    if let index = CommandLine.arguments.firstIndex(of: "--self-test") {
        let output = CommandLine.arguments.count > index + 1 ? CommandLine.arguments[index + 1] : nil
        do {
            try runPickerChecks(outputDirectory: output)
            exit(EXIT_SUCCESS)
        } catch {
            FileHandle.standardError.write(Data("native self-test: \(error)\n".utf8))
            exit(EXIT_FAILURE)
        }
    }
    let delegate = SamplerDelegate()
    application.delegate = delegate
    application.run()
}
