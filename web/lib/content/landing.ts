import { DEMO_PATH } from "@/lib/site";

export const landing = {
  hero: {
    h1: "Deliver the photos. Get paid. Get reviewed.",
    sub: "Photobooth gives every client a private link to preview, approve and pay for their photos with M-PESA. The originals unlock the moment the payment lands.",
    primaryCta: { label: "Get early access", href: "#early-access" },
    secondaryCta: { label: "See what your clients see", href: DEMO_PATH },
    note: "Free to start. Made for photographers in Kenya.",
  },
  problems: {
    heading: "Sound familiar?",
    items: [
      {
        title: "Photos go out late, or not at all.",
        body: "A busy week and a forgotten shoot. Clients wait, then chase you.",
      },
      {
        title: "Not every client has WhatsApp.",
        body: "You juggle email, chat apps and links, depending on what each client prefers.",
      },
      {
        title: "You hand over the originals and hope to get paid.",
        body: "Once the files are sent you lose your leverage and spend days chasing payment.",
      },
      {
        title: "Your best work has nowhere to be reviewed.",
        body: "Happy clients rarely leave a review unless you make it easy.",
      },
    ],
  },
  howItWorks: {
    heading: "From shoot to paid in four steps",
    steps: [
      {
        title: "Upload.",
        body: "Add the shoot and upload your edits at full quality. We keep the originals untouched and create watermarked previews.",
      },
      {
        title: "Send the link.",
        body: "Your client gets a private link by SMS or email. No account, no app.",
      },
      {
        title: "Client approves and pays.",
        body: "They browse the previews, approve, and pay with an M-PESA prompt on their phone.",
      },
      {
        title: "Originals unlock.",
        body: "The watermark lifts and they download every photo in original quality. A review request follows.",
      },
    ],
    demoLink: { label: "Try the client page", href: DEMO_PATH },
  },
  features: {
    heading: "Built around how you actually work",
    items: [
      {
        title: "Original quality, every time.",
        body: "Clients download your exact files, not a compressed copy.",
      },
      {
        title: "Paid before the originals leave your hands.",
        body: "Originals unlock only after the payment is confirmed.",
      },
      {
        title: "Links that reach the right client.",
        body: "Every shoot belongs to a client record and every link is private and unguessable.",
      },
      {
        title: "Never forget a delivery.",
        body: "Track each shoot from shot to delivered, with reminders when one runs late.",
      },
      {
        title: "Reviews that build your name.",
        body: "Ask for a rating right after delivery and show the best ones on your public profile.",
      },
      {
        title: "Storage that tidies itself.",
        body: "Photos expire after the period you choose, and clients get a warning first.",
      },
    ],
  },
  pricing: {
    heading: "Free to start. Pay when you get paid.",
    note: "Early access pricing may change as we learn from photographers.",
  },
  testimonials: {
    heading: "What photographers say",
  },
  faq: {
    heading: "Questions",
    items: [
      {
        q: "Do my clients need an account?",
        a: "No. They open a private link on any phone.",
      },
      {
        q: "When do clients get the originals?",
        a: "After their M-PESA payment is confirmed. Before that they see watermarked previews only.",
      },
      {
        q: "How do clients pay?",
        a: "With an M-PESA prompt on their phone. Card payments may come later.",
      },
      {
        q: "Who owns the photos?",
        a: "You do. Clients can only open their own shoot.",
      },
      {
        q: "How long are photos kept?",
        a: "For the period set by your plan. Clients get a warning before anything expires.",
      },
      {
        q: "How does Photobooth make money?",
        a: "A small commission on paid deliveries, plus optional paid plans with more storage.",
      },
      {
        q: "Is it available now?",
        a: "Not yet. We are inviting photographers in small groups.",
      },
    ],
  },
  earlyAccess: {
    id: "early-access",
    heading: "Get early access",
    text: "Tell us a little about your work. We will invite photographers in small groups and ask for your feedback as we build.",
    success: "Thank you. We will be in touch.",
    demoLink: { label: "Or try the client page first", href: DEMO_PATH },
  },
} as const;

export const photographyTypes = [
  "Weddings",
  "Events",
  "Portraits",
  "Commercial",
  "Other",
] as const;

export const shootsPerMonthOptions = [
  "1 to 5",
  "6 to 15",
  "16 to 30",
  "more than 30",
] as const;
