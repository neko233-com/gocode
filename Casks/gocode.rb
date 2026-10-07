cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.16.0"
  sha256 arm: "69f7a6c0ec566f0f57923add0b4acaf8e10c2d33ea6af12592f43a8636949f48",
         intel: "5c6b4e5107a2600d32492afc7d57b4876c57ecf5cbb4364c5e5874ccba999695"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
