export default function PrivacyPage() {
  return (
    <div className="container max-w-3xl py-12 px-4 md:px-6">
      <h1 className="text-3xl font-bold tracking-tight mb-6">Privacy Policy</h1>
      <p className="text-muted-foreground mb-8">Last updated: {new Date().toLocaleDateString()}</p>
      
      <div className="space-y-6 text-sm text-foreground/80">
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">1. Information We Collect</h2>
          <p className="mb-2">We collect information to provide better services to our users. The types of personal information we may collect include:</p>
          <ul className="list-disc pl-6 space-y-1">
            <li>Information you provide directly to us when creating an account or communicating with us.</li>
            <li>Content you upload, including video files and YouTube URLs, for processing.</li>
            <li>Information retrieved from connected social media accounts (like channel names or IDs) when you authorize OAuth integration.</li>
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">2. How We Use Information</h2>
          <p className="mb-2">We use the information we collect to:</p>
          <ul className="list-disc pl-6 space-y-1">
            <li>Provide, maintain, and improve our video clipping services.</li>
            <li>Process and generate video clips utilizing AI models (like Gemini and WhisperX) within our controlled infrastructure.</li>
            <li>Enable you to publish your generated clips directly to your selected social media platforms (if authorized).</li>
          </ul>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">3. Third-Party Integrations and OAuth</h2>
          <p>When you choose to connect third-party platforms (like YouTube, TikTok, or Instagram) to Coclip, we receive secure access tokens through OAuth. We use these tokens strictly for the purpose of authorizing actions you initiate, such as uploading a video on your behalf. We **never** share your login credentials or access tokens with unauthorized third parties. You can disconnect these services at any time from your account settings.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">4. Data Storage and Security</h2>
          <p>We implement industry-standard security measures (including AES-256-GCM encryption for access tokens) to protect your personal information and uploaded content. Uploaded videos and generated clips are temporarily stored for processing and downloading purposes and are subject to routine cleanup procedures.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">5. Data Sharing</h2>
          <p>We do not sell your personal data. We only share information with third parties when it is strictly necessary to provide our service (such as using an external LLM API for analysis) or when required by law.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">6. Contact Us</h2>
          <p>If you have any questions about this Privacy Policy, please contact the developer or administrator of this Coclip instance.</p>
        </section>
      </div>
    </div>
  );
}
