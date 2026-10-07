cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.14.0"
  sha256 arm: "541819f684f41ea1d55b5dc0fe7201a584eb674095f58258979c269a81d9e2c4",
         intel: "41c244f15aa9e1843985548097c3ae98b490d818ccb5bfcc5a15863524ebf5d6"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
