import java.awt.*; import java.awt.font.*; import java.awt.geom.*; import java.nio.file.*;
public class One { public static void main(String[] a) throws Exception {
  FontRenderContext frc = new FontRenderContext(new AffineTransform(), false, false);
  Font d = new Font("Dialog", Font.PLAIN, 8), l = new Font("Liberation Sans", Font.PLAIN, 8);
  for (String line : Files.readAllLines(Path.of(a[0]))) { String[] f = line.split("\t");
    double s=0; for (char c: f[1].toCharArray()) s+=Math.max(d.getStringBounds(""+c,frc).getWidth(), l.getStringBounds(""+c,frc).getWidth());
    System.out.printf("%-22s %-34s файл %6s  таблица %4.0f  %s%n", f[0], f[1], f[2], s, Math.abs(s-Double.parseDouble(f[2]))<0.5?"=":"≠"); } } }
