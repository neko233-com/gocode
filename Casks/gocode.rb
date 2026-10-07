cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.23.0"
  sha256 arm: "f59a326f8ce92e9405b22b2cbe9e15cfa7fb85120a3886772acf0d8f7a5dcc6c",
         intel: "6780fdc5df8131c6d7bbcf624ea295c61772b534d5d9f53b649b3a15a9c865fa"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
