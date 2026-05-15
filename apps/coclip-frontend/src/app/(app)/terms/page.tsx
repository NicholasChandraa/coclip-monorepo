export default function TermsPage() {
  return (
    <div className="container max-w-3xl py-12 px-4 md:px-6">
      <h1 className="text-3xl font-bold tracking-tight mb-6">Terms of Service</h1>
      <p className="text-muted-foreground mb-8">Last updated: {new Date().toLocaleDateString()}</p>
      
      <div className="space-y-6 text-sm text-foreground/80">
        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">1. Acceptance of Terms</h2>
          <p>By accessing and using Coclip ("the Service"), you agree to be bound by these Terms of Service. If you do not agree to these terms, please do not use the Service.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">2. Description of Service</h2>
          <p>Coclip is an AI-powered automated video clipping tool that helps users generate short-form videos from longer content and optionally upload them to supported social media platforms.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">3. User Content</h2>
          <p>You retain all rights to any audio, video, or other materials you process using the Service ("User Content"). However, by using the Service, you grant us the necessary permissions to process, edit, and temporarily store your User Content solely for the purpose of providing the Service requested by you.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">4. Third-Party Services Integration</h2>
          <p>Our Service integrates with third-party platforms (like YouTube, TikTok, etc.). By using these integrations, you agree to adhere to their respective Terms of Service. We are not responsible for the availability, functionality, or policies of these third-party platforms.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">5. Limitations of Liability</h2>
          <p>In no event shall Coclip, its developers, or affiliates be liable for any indirect, incidental, special, consequential or punitive damages, including without limitation, loss of profits, data, use, goodwill, or other intangible losses, resulting from your access to or use of or inability to access or use the Service.</p>
        </section>

        <section>
          <h2 className="text-xl font-semibold text-foreground mb-3">6. Changes to Terms</h2>
          <p>We reserve the right to modify or replace these Terms at any time. We will provide notice of any significant changes. Your continued use of the Service after any changes constitutes acceptance of the new Terms.</p>
        </section>
      </div>
    </div>
  );
}
