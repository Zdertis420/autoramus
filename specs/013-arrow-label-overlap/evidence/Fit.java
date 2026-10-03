import java.awt.*;
import java.awt.font.*;
import java.awt.geom.*;
import java.text.*;
public class Fit {
    public static void main(String[] a) {
        FontRenderContext frc = new FontRenderContext(new AffineTransform(), false, false);
        String[] probes = {"вход2", "контроль", "Детали изделия", "Швеёно-вышивальгая машинка"};
        for (String f : new String[]{"Dialog", "Liberation Sans"})
        for (float size : new float[]{7f, 7.5f, 8f, 10f}) {
            Font font = new Font(f, Font.PLAIN, 10).deriveFont(size);
            AttributedString s = new AttributedString("Ая"); s.addAttribute(TextAttribute.FONT, font);
            TextLayout tl = new TextLayout(s.getIterator(), frc);
            System.out.printf("%-16s %4.1f  высота %.5f ", f, size, tl.getAscent()+tl.getDescent());
            for (String p : probes) System.out.printf(" %s=%.2f", p, font.getStringBounds(p, frc).getWidth());
            System.out.println();
        }
    }
}
