using System.Collections;
using System.Collections.Generic;
using UnityEngine;

public class SpeedLines : MonoBehaviour
{
    public GameObject line;
    public int count;
    public float offset, minY, maxY, z;
    public Transform cam;
    GameObject[] left, right;

    private void Start()
    {
        left = new GameObject[count];
        right = new GameObject[count];

        for (int i = 0; i < count; i++)
        {
            left[i] = makeNewLeft();
            right[i] = makeNewRight();
        }
    }

    GameObject makeNewLeft()
    {
        GameObject x = Instantiate(line);
        x.transform.parent = transform.parent;
        x.transform.localRotation = cam.transform.localRotation;
        x.transform.localPosition = new Vector3(cam.localPosition.x - offset, Random.Range(minY, maxY), z);
        return x;
    }

    GameObject makeNewRight()
    {
        GameObject x = Instantiate(line);
        x.transform.parent = transform.parent;
        x.transform.localRotation = cam.transform.localRotation;
        x.transform.localPosition = new Vector3(cam.localPosition.x + offset, Random.Range(minY, maxY), z);
        return x;
    }

    private void Update()
    {


        for (int i = count - 1; i >= 0; i--)
            if (right[i] == null) right[i] = makeNewRight();
        for (int i = count - 1; i >= 0; i--)
            if (left[i] == null) left[i] = makeNewLeft();
    }
}
