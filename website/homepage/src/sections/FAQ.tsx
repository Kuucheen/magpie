const faqs = [
  {
    question: 'What is Magpie?',
    answer:
      'Magpie is a self-hosted proxy manager that scrapes public proxies, checks health, scores reputation, and provides rotating endpoints through a web dashboard.',
  },
  {
    question: 'How do I install Magpie?',
    answer:
      'Use the one-command installer for macOS/Linux or Windows PowerShell, or run Docker Compose from the repository.',
  },
  {
    question: 'Does Magpie require Docker?',
    answer:
      'The recommended installation uses Docker Desktop or Docker Engine with Compose, and local development runs services with Docker as well.',
  },
  {
    question: 'What is PROXY_ENCRYPTION_KEY and why is it important?',
    answer:
      'It encrypts stored proxy secrets. If you change it after first run, previously stored secrets cannot be decrypted.',
  },
  {
    question: 'Which proxy protocols are supported?',
    answer:
      'HTTP, HTTPS, SOCKS4, and SOCKS5 proxies are supported, plus TCP and QUIC/HTTP3 transport protocols.',
  },
  {
    question: 'Is Magpie open source?',
    answer: 'Yes. Magpie is licensed under AGPL-3.0.',
  },
];

export default function FAQ() {
  return (
    <section className="relative w-full py-24 lg:py-32 bg-[#0e0e0e] overflow-hidden">
      <div className="relative z-10 container mx-auto px-4 sm:px-6 lg:px-8 xl:px-12">
        <div className="text-center mb-12 space-y-4">
          <h2 className="text-3xl sm:text-4xl lg:text-5xl font-bold text-white">
            Frequently Asked Questions
          </h2>
          <p className="text-lg text-[#a0a0a0] max-w-2xl mx-auto">
            Quick answers about installation, security, and supported protocols.
          </p>
        </div>

        <div className="max-w-4xl mx-auto space-y-4">
          {faqs.map((faq) => (
            <details
              key={faq.question}
              className="group rounded-xl border border-[#2a2a2a] bg-[#1a1a1a] p-5 transition-colors duration-300 open:border-[#3fa37a]/50"
            >
              <summary className="cursor-pointer list-none text-white font-semibold flex items-center justify-between">
                <span>{faq.question}</span>
                <span className="text-[#3fa37a] transition-transform duration-300 group-open:rotate-45">
                  +
                </span>
              </summary>
              <p className="mt-3 text-[#a0a0a0] leading-relaxed">{faq.answer}</p>
            </details>
          ))}
        </div>
      </div>
    </section>
  );
}
