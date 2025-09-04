#!/usr/bin/env python3
"""
Create a simple test PDF with various text scenarios to test extraction quality
"""

from reportlab.pdfgen import canvas
from reportlab.lib.pagesizes import letter
import os

def create_test_pdf(filename):
    c = canvas.Canvas(filename, pagesize=letter)
    width, height = letter
    
    # Page 1 - Simple text
    c.drawString(100, height - 100, "Page 1: Simple Text Testing")
    c.drawString(100, height - 140, "This is a normal paragraph with standard text.")
    c.drawString(100, height - 160, "It contains multiple sentences to test basic extraction.")
    c.drawString(100, height - 180, "Numbers: 12345, Symbols: !@#$%, Mixed: Text123")
    
    # Add some spaced text (common OCR artifact)
    c.drawString(100, height - 220, "S p a c e d   t e x t   l i k e   O C R   m i g h t   p r o d u c e")
    
    # Add text with different cases
    c.drawString(100, height - 260, "CamelCaseText and normal text mixed together")
    c.drawString(100, height - 280, "ALL CAPS TEXT AND normal text")
    
    # Add some bullet points
    c.drawString(100, height - 320, "• First bullet point")
    c.drawString(100, height - 340, "• Second bullet point with more text content")
    c.drawString(100, height - 360, "• Third point")
    
    c.showPage()
    
    # Page 2 - Complex layout
    c.drawString(100, height - 100, "Page 2: Complex Layout Testing")
    
    # Two column simulation
    c.drawString(100, height - 140, "Left Column:")
    c.drawString(100, height - 160, "This text should be in")
    c.drawString(100, height - 180, "the left column area")
    c.drawString(100, height - 200, "with multiple lines")
    
    c.drawString(350, height - 140, "Right Column:")
    c.drawString(350, height - 160, "This text should be in")
    c.drawString(350, height - 180, "the right column area")
    c.drawString(350, height - 200, "with different content")
    
    # Table-like content
    c.drawString(100, height - 280, "Table Data:")
    c.drawString(100, height - 300, "Name        Age    City")
    c.drawString(100, height - 320, "John Doe    25     New York")
    c.drawString(100, height - 340, "Jane Smith  30     Los Angeles")
    c.drawString(100, height - 360, "Bob Wilson  35     Chicago")
    
    c.showPage()
    
    # Page 3 - Edge cases
    c.drawString(100, height - 100, "Page 3: Edge Cases and Special Characters")
    c.drawString(100, height - 140, "Unicode: café résumé naïve coöperate")
    c.drawString(100, height - 160, "Accents: àáâãäåæçèéêëìíîïñòóôõöøùúûüý")
    c.drawString(100, height - 180, "Math: α β γ δ ε π Σ ∑ ∫ ∞ ≤ ≥ ≠ ±")
    c.drawString(100, height - 200, 'Quotes: "smart quotes" \'single quotes\' «guillemets»')
    c.drawString(100, height - 220, "Em dash—en dash–hyphen-")
    c.drawString(100, height - 240, "Ellipsis… and three dots...")
    
    # Very long line to test word wrapping
    long_text = "This is a very long line of text that should extend beyond the normal margins and might cause issues with text extraction if the PDF parser doesn't handle line breaks and word boundaries correctly."
    c.drawString(100, height - 280, long_text[:80])
    c.drawString(100, height - 300, long_text[80:])
    
    c.save()
    print(f"Created test PDF: {filename}")

if __name__ == "__main__":
    create_test_pdf("test_document.pdf")