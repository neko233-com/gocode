cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.22.0"
  sha256 arm: "1ee673c6a49ed3cf7d05cceef71a19f6145b1804b2cda5c449ce0030d294385f",
         intel: "a8a5af51079c058957c6940e165e2955e85e9c50cfc9cd8b6b56588ca9856e8b"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
