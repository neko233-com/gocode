cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.6.0"
  sha256 arm: "bb7bfe0caca3ee5a1fe3fa7f66896425e874482eb4434e0088720db2b1567530",
         intel: "07c56c5c3562f2083e2875b106e46de46c985786d8aab4ed70dc86803edf8f68"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
