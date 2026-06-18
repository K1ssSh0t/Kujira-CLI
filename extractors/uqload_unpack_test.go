package extractors

import "testing"

func TestUnpackPacked(t *testing.T) {
	js := `eval(function(p,a,c,k,e,r){e=function(c){return c.toString(a)};` +
		`if(!''.replace(/^/,String)){while(c--)r[c.toString(a)]=k[c]||c.toString(a);` +
		`k=[function(e){return r[e]}];e=function(){return'\\w+'};c=1};` +
		`while(c--)if(k[c])p=p.replace(new RegExp('\\b'+e(c)+'\\b','g'),k[c]);return p}` +
		`('2("3",{4:"5"})',6,7,'||jwplayer|setup|file|http://cdn.example.com/v.mp4|x'.split('|')))`
	out, err := unpackPacked(js)
	if err != nil {
		t.Fatalf("unpack error: %v", err)
	}
	t.Logf("unpacked: %s", out)
	if got := reUqloadFile.FindStringSubmatch(out); len(got) < 2 || got[1] != "http://cdn.example.com/v.mp4" {
		t.Fatalf("file url not found, got %v", got)
	}
}

func TestUnbaseHighRadix(t *testing.T) {
	// radix 62: "10" => 62
	if v := unbase("10", 62); v != 62 {
		t.Fatalf("expected 62, got %d", v)
	}
	// radix 36: "z" => 35
	if v := unbase("z", 36); v != 35 {
		t.Fatalf("expected 35, got %d", v)
	}
}
