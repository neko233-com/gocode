cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.8.0"
  sha256 arm: "f276be85a3cb0f6b13f2a5a5cefbb65c3e0c183c5e421b6f6c897e810bfa0df0",
         intel: "2fd4c4401e07db876d48e8c520cebeff54903e07f769dc76644fa65153ae0cdb"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
