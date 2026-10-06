cask "gocode" do
  arch arm: "arm64", intel: "amd64"
  version "0.11.0"
  sha256 arm: "86f656fe8afc0d9851df14a8a24ce31af415cb460a2733346e91dfc6959caa10",
         intel: "3eb44831fc8c3f6bee479bbc60ce88322560773a9d69bc1d561a3fbc312215d1"
  url "https://github.com/neko233-com/gocode/releases/download/v#{version}/gocode-#{version}-darwin-#{arch}.tar.gz"
  name "gocode"
  desc "Native Go desktop editor powered by godesktop"
  homepage "https://github.com/neko233-com/gocode"
  depends_on macos: ">= :ventura"
  app "gocode.app"
  binary "#{appdir}/gocode.app/Contents/MacOS/gocode"
end
