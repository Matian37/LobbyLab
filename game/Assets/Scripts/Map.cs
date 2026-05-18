using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class Map : MonoBehaviour
{
    Vector3 offset;
    Vector3 velocity;
    bool drag = false;
    public float hamulec = 0.5f;
    public Camera cam;
    public Vector3 defPos, defScale;
    public RectTransform rt;
    public float nin, nax;
    public float scrollSpeed = 2;
    public RectTransform background;

    private void Start()
    {
        transform.localPosition = defPos;
        transform.localScale = defScale;
    }

    private void Update()
    {
        if (drag)
        {
            velocity = Input.mousePosition - offset - transform.position;
            transform.position = ClampPosition(Input.mousePosition - offset);
        }
        else
        {
            if (velocity.magnitude > 0.1f)
            {
                transform.position = ClampPosition(transform.position + velocity);
                velocity *= hamulec;
            }
        }

        if (Input.GetAxis("Mouse ScrollWheel") != 0f)
        {
            float elo = Input.GetAxis("Mouse ScrollWheel") * scrollSpeed;
            transform.localScale += new Vector3(elo, elo, elo);
            float real = Mathf.Clamp(transform.localScale.x, nin, nax);
            transform.localScale = new Vector3(real, real, real);
            transform.position = ClampPosition(transform.position);
        }
    }

    public void UnClick()
    {
        drag = false;
    }

    public void Click()
    {
        velocity = Vector3.zero;
        offset = Input.mousePosition - transform.position;
        drag = true;
    }

    Vector3 ClampPosition(Vector3 targetPos)
    {
        // pobierz rogi viewportu
        Vector3[] viewportCorners = new Vector3[4];
        rt.GetWorldCorners(viewportCorners);

        // pobierz rogi mapy po przesuniêciu
        Vector3[] mapCorners = new Vector3[4];
        background.GetWorldCorners(mapCorners);

        Vector3 offfset = targetPos - GetComponent<RectTransform>().position; // ró¿nica gdzie chcemy przesun¹æ
        for (int i = 0; i < 4; i++)
            mapCorners[i] += offfset;

        // clamp – sprawdzamy ¿eby rogi mapy by³y w œrodku viewportu
        float maxX = viewportCorners[0].x - (mapCorners[0].x - targetPos.x);
        float minX = viewportCorners[2].x - (mapCorners[2].x - targetPos.x);
        float maxY = viewportCorners[0].y - (mapCorners[0].y - targetPos.y);
        float minY = viewportCorners[2].y - (mapCorners[2].y - targetPos.y);

        targetPos.x = Mathf.Clamp(targetPos.x, minX, maxX);
        targetPos.y = Mathf.Clamp(targetPos.y, minY, maxY);

        return targetPos;
    }
}
