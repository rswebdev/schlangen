import SwiftUI
import CoreImage.CIFilterBuiltins

struct QRCodeView: View {
    let url: String

    var body: some View {
        if let image = generateQRCode(from: url) {
            Image(uiImage: image)
                .interpolation(.none)
                .resizable()
                .scaledToFit()
                .padding(16)
        } else {
            VStack {
                Image(systemName: "qrcode")
                    .font(.system(size: 60))
                    .foregroundColor(.secondary)
                Text("QR unavailable")
                    .font(.caption)
                    .foregroundColor(.secondary)
            }
        }
    }

    private func generateQRCode(from string: String) -> UIImage? {
        let context = CIContext()
        let filter = CIFilter.qrCodeGenerator()
        filter.message = Data(string.utf8)
        filter.correctionLevel = "M"

        guard let outputImage = filter.outputImage else { return nil }

        // Scale up the QR code (it's tiny by default)
        let scale = 10.0
        let transformed = outputImage.transformed(by: CGAffineTransform(scaleX: scale, y: scale))

        guard let cgImage = context.createCGImage(transformed, from: transformed.extent) else {
            return nil
        }

        return UIImage(cgImage: cgImage)
    }
}
